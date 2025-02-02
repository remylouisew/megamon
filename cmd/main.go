/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	"github.com/prometheus/client_golang/prometheus/promhttp"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"example.com/megamon/internal/aggregator"
	"example.com/megamon/internal/controller"
	"example.com/megamon/internal/metrics"

	// +kubebuilder:scaffold:imports

	corev1 "k8s.io/api/core/v1"
	jobset "sigs.k8s.io/jobset/api/jobset/v1alpha2"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(jobset.AddToScheme(scheme))

	// +kubebuilder:scaffold:scheme
}

type config struct {
	AggregationInterval          time.Duration
	ReportConfigMapRef           types.NamespacedName
	JobSetEventsConfigMapRef     types.NamespacedName
	JobSetNodeEventsConfigMapRef types.NamespacedName

	DisableNodePoolJobLabelling bool
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var secureMetrics bool
	var enableHTTP2 bool
	var tlsOpts []func(*tls.Config)
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metrics endpoint binds to. "+
		"Use :8443 for HTTPS or :8080 for HTTP")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&secureMetrics, "metrics-secure", true,
		"If set, the metrics endpoint is served securely via HTTPS. Use --metrics-secure=false to use HTTP instead.")
	flag.BoolVar(&enableHTTP2, "enable-http2", false,
		"If set, HTTP/2 will be enabled for the metrics and webhook servers")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	// TODO: Expose as configuration.
	cfg := config{
		AggregationInterval: 10 * time.Second,
		ReportConfigMapRef: types.NamespacedName{
			Namespace: "megamon-system",
			Name:      "megamon-report",
		},
		JobSetNodeEventsConfigMapRef: types.NamespacedName{
			Namespace: "megamon-system",
			Name:      "megamon-jobset-node-events",
		},
		JobSetEventsConfigMapRef: types.NamespacedName{
			Namespace: "megamon-system",
			Name:      "megamon-jobset-events",
		},
		DisableNodePoolJobLabelling: true,
	}

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// if the enable-http2 flag is false (the default), http/2 should be disabled
	// due to its vulnerabilities. More specifically, disabling http/2 will
	// prevent from being vulnerable to the HTTP/2 Stream Cancellation and
	// Rapid Reset CVEs. For more information see:
	// - https://github.com/advisories/GHSA-qppj-fm5r-hxr3
	// - https://github.com/advisories/GHSA-4374-p667-p6c8
	disableHTTP2 := func(c *tls.Config) {
		setupLog.Info("disabling http/2")
		c.NextProtos = []string{"http/1.1"}
	}

	if !enableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}

	webhookServer := webhook.NewServer(webhook.Options{
		TLSOpts: tlsOpts,
	})

	// Metrics endpoint is enabled in 'config/default/kustomization.yaml'. The Metrics options configure the server.
	// More info:
	// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.0/pkg/metrics/server
	// - https://book.kubebuilder.io/reference/metrics.html
	metricsServerOptions := metricsserver.Options{
		// Disable in favor of the otel metrics server.
		BindAddress:   "0", // metricsAddr,
		SecureServing: secureMetrics,
		// TODO(user): TLSOpts is used to allow configuring the TLS config used for the server. If certificates are
		// not provided, self-signed certificates will be generated by default. This option is not recommended for
		// production environments as self-signed certificates do not offer the same level of trust and security
		// as certificates issued by a trusted Certificate Authority (CA). The primary risk is potentially allowing
		// unauthorized access to sensitive metrics data. Consider replacing with CertDir, CertName, and KeyName
		// to provide certificates, ensuring the server communicates using trusted and secure certificates.
		TLSOpts: tlsOpts,
	}

	if secureMetrics {
		// FilterProvider is used to protect the metrics endpoint with authn/authz.
		// These configurations ensure that only authorized users and service accounts
		// can access the metrics endpoint. The RBAC are configured in 'config/rbac/kustomization.yaml'. More info:
		// https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.0/pkg/metrics/filters#WithAuthenticationAndAuthorization
		metricsServerOptions.FilterProvider = filters.WithAuthenticationAndAuthorization
	}

	// Only watch JobSet leader pods so that Jobs can be bound to the node
	// pools that they are scheduled on.
	//
	// jobset.sigs.k8s.io/jobset-name (exists)
	// batch.kubernetes.io/job-completion-index: "0"
	//
	jobsetPodSelector, err := labels.Parse("jobset.sigs.k8s.io/jobset-name, batch.kubernetes.io/job-completion-index=0")
	if err != nil {
		setupLog.Error(err, "unable to create jobset pod selector")
		os.Exit(1)
	}
	// Only watch Pods that are already scheduled to a Node.
	scheduledPodSelector, err := fields.ParseSelector("spec.nodeName!=")
	if err != nil {
		setupLog.Error(err, "unable to create scheduled pod selector")
		os.Exit(1)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsServerOptions,
		WebhookServer:          webhookServer,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "fd0479f1.example.com",
		Cache: cache.Options{
			ByObject: map[client.Object]cache.ByObject{
				&corev1.Pod{}: {
					Label: jobsetPodSelector,
					Field: scheduledPodSelector,
				},
			},
		},
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&controller.JobSetReconciler{
		Disabled: false,
		//JobSetEventsConfigMapRef: cfg.JobSetEventsConfigMapRef,
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "JobSet")
		os.Exit(1)
	}
	if err = (&controller.NodeReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Node")
		os.Exit(1)
	}
	if !cfg.DisableNodePoolJobLabelling {
		if err = (&controller.PodReconciler{
			Client: mgr.GetClient(),
			Scheme: mgr.GetScheme(),
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "Pod")
			os.Exit(1)
		}
	}
	// +kubebuilder:scaffold:builder

	ctx := ctrl.SetupSignalHandler()

	agg := &aggregator.Aggregator{
		JobSetEventsConfigMapRef:     cfg.JobSetEventsConfigMapRef,
		JobSetNodeEventsConfigMapRef: cfg.JobSetNodeEventsConfigMapRef,
		Interval:                     cfg.AggregationInterval,
		Client:                       mgr.GetClient(),
		Exporters: map[string]aggregator.Exporter{
			"configmap": &aggregator.ConfigMapExporter{
				Client: mgr.GetClient(),
				Ref:    cfg.ReportConfigMapRef,
				Key:    "report",
			},
			"stdout": &aggregator.StdoutExporter{},
		},
	}
	shutdownMetrics := metrics.Init(agg)
	//mgr.Add(agg)

	// Initial aggregation to populate the initial metrics report.
	// TODO: Verify the readiness check is applied before scraping.
	//if err := agg.Aggregate(ctx); err != nil {
	//	setupLog.Error(err, "failed initial aggregate")
	//}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	// Add readiness check that makes sure that the aggregator is ready.
	// TODO: Validate that GMP waits for Readiness before scraping.
	if err := mgr.AddReadyzCheck("readyz", func(req *http.Request) error {
		if !agg.ReportReady() {
			return errors.New("aggregator report not ready")
		}
		return nil
	}); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	defer shutdownMetrics()
	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", promhttp.Handler())
	metricsServer := http.Server{Handler: metricsMux, Addr: metricsAddr}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		log.Println("starting aggregator")
		defer wg.Done()
		if err := agg.Start(ctx); err != nil {
			setupLog.Error(err, "error serving metrics server")
			os.Exit(1)
		}
	}()

	wg.Add(1)
	go func() {
		log.Println("starting metrics server")
		defer wg.Done()
		if err := metricsServer.ListenAndServe(); err != nil {
			if errors.Is(err, http.ErrServerClosed) {
				setupLog.Info("metrics server closed")
			} else {
				setupLog.Error(err, "error serving metrics server")
				os.Exit(1)
			}
		}
	}()

	setupLog.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
	}
	metricsServer.Shutdown(context.Background())

	setupLog.Info("waiting for all goroutines to stop")
	wg.Wait()
	setupLog.Info("all goroutines stopped, exiting")
}
