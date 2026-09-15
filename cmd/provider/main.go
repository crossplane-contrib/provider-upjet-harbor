package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/alecthomas/kingpin/v2"
	changelogsv1alpha1 "github.com/crossplane/crossplane-runtime/v2/apis/changelogs/proto/v1alpha1"
	xpcontroller "github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/feature"
	"github.com/crossplane/crossplane-runtime/v2/pkg/gate"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/customresourcesgate"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/statemetrics"
	tjcontroller "github.com/crossplane/upjet/v2/pkg/controller"
	"github.com/crossplane/upjet/v2/pkg/controller/conversion"
	"github.com/crossplane/upjet/v2/pkg/terraform"
	harborprovider "github.com/goharbor/terraform-provider-harbor/provider"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	authv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	apisCluster "github.com/crossplane-contrib/provider-upjet-harbor/apis/cluster"
	apisNamespaced "github.com/crossplane-contrib/provider-upjet-harbor/apis/namespaced"
	"github.com/crossplane-contrib/provider-upjet-harbor/config"
	resolverapis "github.com/crossplane-contrib/provider-upjet-harbor/internal/apis"
	"github.com/crossplane-contrib/provider-upjet-harbor/internal/clients"
	controllerCluster "github.com/crossplane-contrib/provider-upjet-harbor/internal/controller/cluster"
	controllerNamespaced "github.com/crossplane-contrib/provider-upjet-harbor/internal/controller/namespaced"
	"github.com/crossplane-contrib/provider-upjet-harbor/internal/features"
	"github.com/crossplane-contrib/provider-upjet-harbor/internal/version"
)

const (
	webhookTLSCertDirEnvVar = "WEBHOOK_TLS_CERT_DIR"
	tlsServerCertDirEnvVar  = "TLS_SERVER_CERTS_DIR"
	certsDirEnvVar          = "CERTS_DIR"
	tlsServerCertDir        = "/tls/server"
)

//nolint:gocyclo // The enable-secret-cache branch below is the standard crossplane-runtime v2.4.0 wiring, copy-pasted as-is rather than extracted into a helper.
func main() {
	var (
		app                     = kingpin.New(filepath.Base(os.Args[0]), "Terraform based Crossplane provider for Harbor").DefaultEnvars()
		debug                   = app.Flag("debug", "Run with debug logging.").Short('d').Bool()
		syncPeriod              = app.Flag("sync", "Controller manager sync period such as 300ms, 1.5h, or 2h45m").Short('s').Default("1h").Duration()
		pollInterval            = app.Flag("poll", "Poll interval controls how often an individual resource should be checked for drift.").Default("10m").Duration()
		pollStateMetricInterval = app.Flag("poll-state-metric", "State metric recording interval").Default("5s").Duration()
		leaderElection          = app.Flag("leader-election", "Use leader election for the controller manager.").Short('l').Default("false").OverrideDefaultFromEnvar("LEADER_ELECTION").Bool()
		maxReconcileRate        = app.Flag("max-reconcile-rate", "The global maximum rate per second at which resources may be checked for drift from the desired state.").Default("10").Int()

		webhookPort            = app.Flag("webhook-port", "The port the webhook listens on").Default("9443").Envar("WEBHOOK_PORT").Int()
		metricsBindAddress     = app.Flag("metrics-bind-address", "The address the metrics server listens on").Default(":8080").Envar("METRICS_BIND_ADDRESS").String()
		healthProbeBindAddress = app.Flag("health-probe-bind-addr", "The address the health/readiness probe server listens on").Default(":8081").Envar("HEALTH_PROBE_BIND_ADDRESS").String()
		changelogsSocketPath   = app.Flag("changelogs-socket-path", "Path for changelogs socket (if enabled)").Default("/var/run/changelogs/changelogs.sock").Envar("CHANGELOGS_SOCKET_PATH").String()

		enableManagementPolicies = app.Flag("enable-management-policies", "Enable support for Management Policies.").Default("true").Envar("ENABLE_MANAGEMENT_POLICIES").Bool()
		enableChangeLogs         = app.Flag("enable-changelogs", "Enable support for capturing change logs during reconciliation.").Default("false").Envar("ENABLE_CHANGE_LOGS").Bool()
		enableSecretCache        = app.Flag("enable-secret-cache", "Enable caching of Secrets in the controller manager's informer cache.").Default("true").Envar("ENABLE_SECRET_CACHE").Bool()

		certsDirSet = false
		// we record whether the command-line option "--certs-dir" was supplied
		// in the registered PreAction for the flag.
		certsDir = app.Flag("certs-dir", "The directory that contains the server key and certificate.").Default(tlsServerCertDir).Envar(certsDirEnvVar).PreAction(func(_ *kingpin.ParseContext) error {
			certsDirSet = true
			return nil
		}).String()
	)

	kingpin.MustParse(app.Parse(os.Args[1:]))

	zl := zap.New(zap.UseDevMode(*debug))
	log := logging.NewLogrLogger(zl.WithName("provider-upjet-harbor"))
	if *debug {
		// The controller-runtime runs with a no-op logger by default. It is
		// *very* verbose even at info level, so we only provide it a real
		// logger when we're running in debug mode.
		ctrl.SetLogger(zl)
	}

	// Currently the jitter is 5% of the poll interval (matches azuread).
	pollJitter := time.Duration(float64(*pollInterval) * 0.05)

	log.Debug("Starting", "sync-period", syncPeriod.String(), "poll-interval", pollInterval.String(), "poll-jitter", pollJitter, "max-reconcile-rate", *maxReconcileRate)

	cfg, err := ctrl.GetConfig()
	kingpin.FatalIfError(err, "Cannot get API server rest config")

	*certsDir = resolveCertsDir(certsDirSet, *certsDir)

	scheme := runtime.NewScheme()
	kingpin.FatalIfError(clientgoscheme.AddToScheme(scheme), "Cannot add client-go APIs to scheme")
	kingpin.FatalIfError(apisCluster.AddToScheme(scheme), "Cannot add cluster-scoped Harbor APIs to scheme")
	kingpin.FatalIfError(resolverapis.BuildScheme(apisCluster.AddToSchemes), "Cannot register the cluster-scoped Harbor APIs with the API resolver's runtime scheme")
	kingpin.FatalIfError(apisNamespaced.AddToScheme(scheme), "Cannot add namespaced Harbor APIs to scheme")
	kingpin.FatalIfError(resolverapis.BuildScheme(apisNamespaced.AddToSchemes), "Cannot register the namespaced Harbor APIs with the API resolver's runtime scheme")
	kingpin.FatalIfError(apiextensionsv1.AddToScheme(scheme), "Cannot add api-extensions APIs to scheme")
	kingpin.FatalIfError(authv1.AddToScheme(scheme), "Cannot add k8s authorization APIs to scheme")

	// When secret caching is disabled, Secrets are fetched directly from the
	// API server on every read instead of being kept in the manager's
	// informer cache, trading memory for API server QPS.
	var clientOpts client.Options
	if !*enableSecretCache {
		clientOpts = client.Options{
			Cache: &client.CacheOptions{DisableFor: []client.Object{&corev1.Secret{}}},
		}
	}

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:           scheme,
		Client:           clientOpts,
		LeaderElection:   *leaderElection,
		LeaderElectionID: "crossplane-leader-election-provider-upjet-harbor",
		Cache: cache.Options{
			SyncPeriod: syncPeriod,
			ByObject: map[client.Object]cache.ByObject{
				&apiextensionsv1.CustomResourceDefinition{}: {
					Transform: customresourcesgate.TransformStripCRDSchema,
				},
			},
		},
		Metrics: metricsserver.Options{
			BindAddress: *metricsBindAddress,
		},
		HealthProbeBindAddress: *healthProbeBindAddress,
		WebhookServer: webhook.NewServer(
			webhook.Options{
				CertDir: *certsDir,
				Port:    *webhookPort,
			}),
		LeaderElectionResourceLock: resourcelock.LeasesResourceLock,
		LeaseDuration:              func() *time.Duration { d := 60 * time.Second; return &d }(),
		RenewDeadline:              func() *time.Duration { d := 50 * time.Second; return &d }(),
	})
	kingpin.FatalIfError(err, "Cannot create controller manager")
	if len(*certsDir) > 0 {
		kingpin.FatalIfError(mgr.AddReadyzCheck("webhook", mgr.GetWebhookServer().StartedChecker()), "Cannot add webhook server readyz checker to controller manager")
	}

	metricRecorder := managed.NewMRMetricRecorder()
	stateMetrics := statemetrics.NewMRStateMetrics()

	metrics.Registry.MustRegister(metricRecorder)
	metrics.Registry.MustRegister(stateMetrics)

	ctx := context.Background()
	sdkProvider := harborprovider.Provider()

	clusterProvider, err := config.GetProvider(ctx, sdkProvider)
	kingpin.FatalIfError(err, "Cannot initialize the cluster-scoped provider configuration")
	namespacedProvider, err := config.GetProviderNamespaced(ctx, sdkProvider)
	kingpin.FatalIfError(err, "Cannot initialize the namespaced provider configuration")

	clusterOpts := tjcontroller.Options{
		Options: xpcontroller.Options{
			Logger:                  log,
			GlobalRateLimiter:       ratelimiter.NewGlobal(*maxReconcileRate),
			PollInterval:            *pollInterval,
			MaxConcurrentReconciles: *maxReconcileRate,
			Features:                &feature.Flags{},
			MetricOptions: &xpcontroller.MetricOptions{
				PollStateMetricInterval: *pollStateMetricInterval,
				MRMetrics:               metricRecorder,
				MRStateMetrics:          stateMetrics,
			},
		},
		Provider:              clusterProvider,
		WorkspaceStore:        terraform.NewWorkspaceStore(log),
		SetupFn:               clients.TerraformSetupBuilder(sdkProvider),
		PollJitter:            pollJitter,
		OperationTrackerStore: tjcontroller.NewOperationStore(log),
	}

	namespacedOpts := tjcontroller.Options{
		Options: xpcontroller.Options{
			Logger:                  log,
			GlobalRateLimiter:       ratelimiter.NewGlobal(*maxReconcileRate),
			PollInterval:            *pollInterval,
			MaxConcurrentReconciles: *maxReconcileRate,
			Features:                &feature.Flags{},
			MetricOptions: &xpcontroller.MetricOptions{
				PollStateMetricInterval: *pollStateMetricInterval,
				MRMetrics:               metricRecorder,
				MRStateMetrics:          stateMetrics,
			},
		},
		Provider:              namespacedProvider,
		WorkspaceStore:        terraform.NewWorkspaceStore(log),
		SetupFn:               clients.TerraformSetupBuilder(sdkProvider),
		PollJitter:            pollJitter,
		OperationTrackerStore: tjcontroller.NewOperationStore(log),
	}

	if *enableManagementPolicies {
		clusterOpts.Features.Enable(features.EnableBetaManagementPolicies)
		namespacedOpts.Features.Enable(features.EnableBetaManagementPolicies)
		log.Info("Beta feature enabled", "flag", features.EnableBetaManagementPolicies)
	}

	if *enableChangeLogs {
		clusterOpts.Features.Enable(feature.EnableAlphaChangeLogs)
		namespacedOpts.Features.Enable(feature.EnableAlphaChangeLogs)
		log.Info("Alpha feature enabled", "flag", feature.EnableAlphaChangeLogs)

		conn, err := grpc.NewClient("unix://"+*changelogsSocketPath, grpc.WithTransportCredentials(insecure.NewCredentials()))
		kingpin.FatalIfError(err, "failed to create change logs client connection at %s", *changelogsSocketPath)

		clo := xpcontroller.ChangeLogOptions{
			ChangeLogger: managed.NewGRPCChangeLogger(
				changelogsv1alpha1.NewChangeLogServiceClient(conn),
				managed.WithProviderVersion(fmt.Sprintf("provider-upjet-harbor:%s", version.Version))),
		}
		clusterOpts.ChangeLogOptions = &clo
		namespacedOpts.ChangeLogOptions = &clo
	}

	// Conversion webhooks are registered eagerly on every replica so that
	// every replica (leader and followers alike) can serve conversion requests.
	// Reconciler setup is deferred to the gate and only runs on the leader.
	if *certsDir != "" {
		kingpin.FatalIfError(controllerCluster.SetupWebhookWithManager(mgr), "Cannot setup cluster-scoped webhooks")
		kingpin.FatalIfError(controllerNamespaced.SetupWebhookWithManager(mgr), "Cannot setup namespaced webhooks")
	}

	canSafeStart, err := canWatchCRD(context.TODO(), mgr)
	kingpin.FatalIfError(err, "SafeStart precheck failed")
	if canSafeStart {
		crdGate := new(gate.Gate[schema.GroupVersionKind])
		clusterOpts.Gate = crdGate
		namespacedOpts.Gate = crdGate
		kingpin.FatalIfError(customresourcesgate.Setup(mgr, xpcontroller.Options{
			Logger:                  log,
			Gate:                    crdGate,
			MaxConcurrentReconciles: 1,
		}), "Cannot setup CRD gate")
		kingpin.FatalIfError(controllerCluster.SetupGated(mgr, clusterOpts), "Cannot setup cluster-scoped Harbor controllers")
		kingpin.FatalIfError(controllerNamespaced.SetupGated(mgr, namespacedOpts), "Cannot setup namespaced Harbor controllers")
	} else {
		log.Info("Provider has missing RBAC permissions for watching CRDs, controller SafeStart capability will be disabled")
		kingpin.FatalIfError(controllerCluster.Setup(mgr, clusterOpts), "Cannot setup cluster-scoped Harbor controllers")
		kingpin.FatalIfError(controllerNamespaced.Setup(mgr, namespacedOpts), "Cannot setup namespaced Harbor controllers")
	}

	kingpin.FatalIfError(conversion.RegisterConversions(clusterOpts.Provider, namespacedOpts.Provider, mgr.GetScheme()), "Cannot initialize the webhook conversion registry")

	kingpin.FatalIfError(mgr.Start(ctrl.SetupSignalHandler()), "Cannot start controller manager")
}

// resolveCertsDir returns the TLS certs directory to use. When no explicit
// certs dir was supplied via the command-line options, the environment
// variables set by Crossplane are used instead: in older XP versions
// WEBHOOK_TLS_CERT_DIR, in newer versions TLS_SERVER_CERTS_DIR.
func resolveCertsDir(certsDirSet bool, certsDir string) string {
	if certsDirSet {
		return certsDir
	}
	// backwards-compatibility concerns
	xpCertsDir := os.Getenv(certsDirEnvVar)
	if xpCertsDir == "" {
		xpCertsDir = os.Getenv(tlsServerCertDirEnvVar)
	}
	if xpCertsDir == "" {
		xpCertsDir = os.Getenv(webhookTLSCertDirEnvVar)
	}
	// we probably don't need this condition but just to be on the
	// safe side, if we are missing any kingpin machinery details...
	if xpCertsDir != "" {
		return xpCertsDir
	}
	return certsDir
}

func canWatchCRD(ctx context.Context, mgr manager.Manager) (bool, error) {
	if err := authv1.AddToScheme(mgr.GetScheme()); err != nil {
		return false, err
	}
	verbs := []string{"get", "list", "watch"}
	for _, verb := range verbs {
		sar := &authv1.SelfSubjectAccessReview{
			Spec: authv1.SelfSubjectAccessReviewSpec{
				ResourceAttributes: &authv1.ResourceAttributes{
					Group:    "apiextensions.k8s.io",
					Resource: "customresourcedefinitions",
					Verb:     verb,
				},
			},
		}
		if err := mgr.GetClient().Create(ctx, sar); err != nil {
			return false, errors.Wrapf(err, "unable to perform RBAC check for verb %s on CustomResourceDefinitions", verbs)
		}
		if !sar.Status.Allowed {
			return false, nil
		}
	}
	return true, nil
}
