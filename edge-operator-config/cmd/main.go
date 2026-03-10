package main

import (
	"flag"
	"os"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	iotv1alpha1 "github.com/jaiderssjgod/edge-operator/api/v1alpha1"
	controller "github.com/jaiderssjgod/edge-operator/internal/controller"
	"github.com/jaiderssjgod/edge-operator/internal/degradation"
	"github.com/jaiderssjgod/edge-operator/internal/heartbeatstore"
	"github.com/jaiderssjgod/edge-operator/internal/heartbeatstore/heartbeatserver"
)

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(iotv1alpha1.AddToScheme(scheme))
}

func main() {
	var (
		metricsAddr          string
		probeAddr            string
		heartbeatAddr        string
		heartbeatTimeoutSecs int
		enableLeaderElection bool
	)

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "")
	flag.StringVar(&heartbeatAddr, "heartbeat-bind-address", ":9090", "")
	flag.IntVar(&heartbeatTimeoutSecs, "heartbeat-timeout-seconds", 30, "")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false, "")
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))
	log := ctrl.Log.WithName("operator")

	hbStore := heartbeatstore.New(time.Duration(heartbeatTimeoutSecs) * time.Second)

	hbServer := heartbeatserver.New(heartbeatAddr, hbStore, log.WithName("heartbeat-server"))
	go hbServer.Start()

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsserver.Options{BindAddress: metricsAddr},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "edge-operator-leader",
	})
	if err != nil {
		log.Error(err, "Unable to start manager")
		os.Exit(1)
	}

	// El índice spec.nodeName se registra dentro de SetupWithManager
	degradationMgr := degradation.New(
		mgr.GetClient(),
		ctrl.Log.WithName("degradation"),
	)

	if err = (&controller.ReducedNodePolicyReconciler{
		Client:             mgr.GetClient(),
		Log:                ctrl.Log.WithName("controllers").WithName("ReducedNodePolicy"),
		Scheme:             mgr.GetScheme(),
		HeartbeatStore:     hbStore,
		DegradationManager: degradationMgr,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "Unable to create controller", "controller", "ReducedNodePolicy")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		log.Error(err, "Unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		log.Error(err, "Unable to set up ready check")
		os.Exit(1)
	}

	log.Info("Starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		log.Error(err, "Problem running manager")
		os.Exit(1)
	}
}