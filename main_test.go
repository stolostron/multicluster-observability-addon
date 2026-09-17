// Copyright (c) Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project
// Licensed under the Apache License 2.0

package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-logr/logr"
	configv1 "github.com/openshift/api/config/v1"
	libgocrypto "github.com/openshift/library-go/pkg/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	authenticationv1 "k8s.io/api/authentication/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	utilflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/metrics"
	"k8s.io/component-base/metrics/legacyregistry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

func TestNewControllerCommandFlags(t *testing.T) {
	cmd := newControllerCommand()
	for name, defaultValue := range map[string]string{
		"enable-leader-election":    "false",
		"component-namespace":       "",
		"kubeconfig":                "",
		"log-verbosity":             "0",
		"metrics-bind-address":      ":8084",
		"health-probe-bind-address": ":8081",
		"metrics-cert-dir":          "/var/run/secrets/serving-cert",
	} {
		t.Run(name, func(t *testing.T) {
			flag := cmd.Flags().Lookup(name)
			require.NotNil(t, flag)
			assert.Equal(t, defaultValue, flag.DefValue)
			assert.Equal(t, defaultValue, flag.Value.String())
		})
	}
	for _, name := range []string{"listen", "enable-pprof", "pprof-addr"} {
		assert.Nil(t, cmd.Flags().Lookup(name), "removed flag %s", name)
	}
}

func TestNewManagerOptionsDefaults(t *testing.T) {
	opts, err := newManagerOptions(*configv1.TLSProfiles[configv1.TLSProfileIntermediateType])
	require.NoError(t, err)
	assert.False(t, opts.LeaderElection)
	assert.Equal(t, "multicluster-observability-addon-controller", opts.LeaderElectionID)
	assert.Equal(t, "leases", opts.LeaderElectionResourceLock)
	assert.True(t, opts.LeaderElectionReleaseOnCancel)
	for name, tt := range map[string]struct {
		got  *time.Duration
		want time.Duration
	}{
		"lease":    {opts.LeaseDuration, 137 * time.Second},
		"renew":    {opts.RenewDeadline, 107 * time.Second},
		"retry":    {opts.RetryPeriod, 26 * time.Second},
		"shutdown": {opts.GracefulShutdownTimeout, 10 * time.Second},
	} {
		require.NotNil(t, tt.got, name)
		assert.Equal(t, tt.want, *tt.got, name)
	}
	assert.Equal(t, ":8081", opts.HealthProbeBindAddress)
	assert.Equal(t, "0", opts.PprofBindAddress)
	assert.Equal(t, ":8084", opts.Metrics.BindAddress)
	assert.Equal(t, "/var/run/secrets/serving-cert", opts.Metrics.CertDir)
	assert.True(t, opts.Metrics.SecureServing)
	assert.NotNil(t, opts.Metrics.FilterProvider)
	require.Len(t, opts.Metrics.ExtraHandlers, 1, "only legacy metrics, never pprof handlers")
	require.NotNil(t, opts.Metrics.ExtraHandlers["/metrics/legacy"])
	metric := metrics.NewGauge(&metrics.GaugeOpts{Name: "mcoa_test_legacy_registry", Help: "Test legacy registry routing."})
	require.NoError(t, legacyregistry.Register(metric))
	defer legacyregistry.Registerer().Unregister(metric)
	metric.Set(42)
	response := httptest.NewRecorder()
	opts.Metrics.ExtraHandlers["/metrics/legacy"].ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/metrics/legacy", nil))
	assert.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), "mcoa_test_legacy_registry 42\n")
}

func TestNewManagerOptionsTLSPolicy(t *testing.T) {
	for name, profile := range map[string]configv1.TLSProfileSpec{
		"intermediate": *configv1.TLSProfiles[configv1.TLSProfileIntermediateType],
		"modern":       *configv1.TLSProfiles[configv1.TLSProfileModernType],
		"custom IANA": {
			MinTLSVersion: configv1.VersionTLS12,
			Ciphers: []string{
				"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
				"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384",
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			opts, err := newManagerOptions(profile)
			require.NoError(t, err)

			metricsTLS := &tls.Config{NextProtos: []string{"h2", "http/1.1"}}
			for _, configure := range opts.Metrics.TLSOpts {
				configure(metricsTLS)
			}
			wantVersion, err := utilflag.TLSVersion(string(profile.MinTLSVersion))
			require.NoError(t, err)
			assert.Equal(t, wantVersion, metricsTLS.MinVersion)
			if profile.MinTLSVersion == configv1.VersionTLS13 {
				assert.Empty(t, metricsTLS.CipherSuites, "Go does not configure TLS 1.3 cipher suites")
			} else {
				if name != "custom IANA" {
					ciphers, err := utilflag.TLSCipherSuites(libgocrypto.OpenSSLToIANACipherSuites(profile.Ciphers))
					require.NoError(t, err)
					assert.ElementsMatch(t, ciphers, metricsTLS.CipherSuites)
				}
				assert.NotEmpty(t, metricsTLS.CipherSuites)
			}
			if name == "custom IANA" {
				assert.ElementsMatch(t, []uint16{
					tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
					tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
				}, metricsTLS.CipherSuites)
			}
			assert.Equal(t, []string{"http/1.1"}, metricsTLS.NextProtos)
		})
	}
}

func TestMetricsFilterAuthenticationAndAuthorization(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/apis/authentication.k8s.io/v1/tokenreviews":
			var review authenticationv1.TokenReview
			if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&review)) {
				http.Error(w, "invalid TokenReview", http.StatusBadRequest)
				return
			}
			switch review.Spec.Token {
			case "metrics-only", "legacy-only", "unauthorized":
				review.Status.Authenticated = true
				review.Status.User = authenticationv1.UserInfo{Username: review.Spec.Token}
			}
			assert.NoError(t, json.NewEncoder(w).Encode(review))
		case "/apis/authorization.k8s.io/v1/subjectaccessreviews":
			var review authorizationv1.SubjectAccessReview
			if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&review)) {
				http.Error(w, "invalid SubjectAccessReview", http.StatusBadRequest)
				return
			}
			assert.Nil(t, review.Spec.ResourceAttributes)
			if attrs := review.Spec.NonResourceAttributes; assert.NotNil(t, attrs) {
				review.Status.Allowed = attrs.Verb == "get" &&
					((review.Spec.User == "metrics-only" && attrs.Path == "/metrics") ||
						(review.Spec.User == "legacy-only" && attrs.Path == "/metrics/legacy"))
			}
			assert.NoError(t, json.NewEncoder(w).Encode(review))
		default:
			t.Errorf("unexpected Kubernetes API request: %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer api.Close()

	opts, err := newManagerOptions(*configv1.TLSProfiles[configv1.TLSProfileIntermediateType])
	require.NoError(t, err)
	require.NotNil(t, opts.Metrics.FilterProvider)
	filter, err := opts.Metrics.FilterProvider(&rest.Config{Host: api.URL}, api.Client())
	require.NoError(t, err)

	for _, tt := range []struct {
		name   string
		token  string
		path   string
		status int
	}{
		{name: "anonymous", path: "/metrics", status: http.StatusUnauthorized},
		// The real filter reports invalid bearer tokens as authentication errors (500).
		{name: "invalid token", token: "invalid", path: "/metrics", status: http.StatusInternalServerError},
		{name: "unauthorized identity", token: "unauthorized", path: "/metrics", status: http.StatusForbidden},
		{name: "authorized metrics", token: "metrics-only", path: "/metrics", status: http.StatusOK},
		{name: "anonymous legacy", path: "/metrics/legacy", status: http.StatusUnauthorized},
		{name: "unauthorized legacy", token: "unauthorized", path: "/metrics/legacy", status: http.StatusForbidden},
		{name: "authorized legacy", token: "legacy-only", path: "/metrics/legacy", status: http.StatusOK},
		{name: "metrics grant excludes legacy", token: "metrics-only", path: "/metrics/legacy", status: http.StatusForbidden},
		{name: "legacy grant excludes primary", token: "legacy-only", path: "/metrics", status: http.StatusForbidden},
		{name: "metrics grant excludes profiling", token: "metrics-only", path: "/debug/pprof/", status: http.StatusForbidden},
	} {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			// This test handler checks path-specific authorization, not listener routing.
			handler, err := filter(logr.Discard(), http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				called = true
				w.WriteHeader(http.StatusOK)
			}))
			require.NoError(t, err)
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			if tt.token != "" {
				req.Header.Set("Authorization", "Bearer "+tt.token)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, req)
			assert.Equal(t, tt.status, response.Code, response.Body.String())
			assert.Equal(t, tt.status == http.StatusOK, called)
		})
	}
}

func TestNewManagerOptionsRejectsUnsupportedCiphers(t *testing.T) {
	for _, ciphers := range [][]string{nil, {"ECDHE-RSA-AES256-SHA384"}} {
		_, err := newManagerOptions(configv1.TLSProfileSpec{
			MinTLSVersion: configv1.VersionTLS12,
			Ciphers:       ciphers,
		})
		require.ErrorIs(t, err, errNoSupportedTLSCiphers)
	}
}

func TestTLSProfileWatcherReconcile(t *testing.T) {
	for _, tt := range []struct {
		name        string
		initial     configv1.TLSProfileType
		current     configv1.TLSProfileType
		adherence   configv1.TLSAdherencePolicy
		wantRestart bool
	}{
		{name: "unchanged", initial: configv1.TLSProfileIntermediateType, current: configv1.TLSProfileIntermediateType, adherence: configv1.TLSAdherencePolicyStrictAllComponents},
		{name: "tightened", initial: configv1.TLSProfileIntermediateType, current: configv1.TLSProfileModernType, adherence: configv1.TLSAdherencePolicyStrictAllComponents, wantRestart: true},
		{name: "unhonored profile", initial: configv1.TLSProfileIntermediateType, current: configv1.TLSProfileModernType, adherence: configv1.TLSAdherencePolicyNoOpinion},
		{name: "adherence changed", initial: configv1.TLSProfileModernType, current: configv1.TLSProfileModernType, adherence: configv1.TLSAdherencePolicyNoOpinion, wantRestart: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			for _, update := range []bool{false, true} {
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				obj := &configv1.APIServer{
					ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
					Spec: configv1.APIServerSpec{
						TLSSecurityProfile: &configv1.TLSSecurityProfile{Type: tt.current},
						TLSAdherence:       tt.adherence,
					},
				}
				fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
				watcher := newTLSProfileWatcher(fakeClient, *configv1.TLSProfiles[tt.initial], cancel)
				request := ctrl.Request{NamespacedName: client.ObjectKey{Name: "cluster"}}
				if update {
					baseline := obj.DeepCopy()
					baseline.Spec.TLSSecurityProfile.Type = tt.initial
					baseline.Spec.TLSAdherence = configv1.TLSAdherencePolicyStrictAllComponents
					require.NoError(t, fakeClient.Create(t.Context(), baseline))
					_, err := watcher.Reconcile(t.Context(), request)
					require.NoError(t, err)
					require.NoError(t, ctx.Err(), "baseline must not restart")
					obj.ResourceVersion = baseline.ResourceVersion
					require.NoError(t, fakeClient.Update(t.Context(), obj))
				} else {
					require.NoError(t, fakeClient.Create(t.Context(), obj))
				}
				_, err := watcher.Reconcile(t.Context(), request)
				require.NoError(t, err)
				assert.Equal(t, tt.wantRestart, ctx.Err() != nil, "update=%t", update)
			}
		})
	}
}

func TestTLSProfileWatcherAdherenceOnlyChange(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	obj := &configv1.APIServer{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
		Spec: configv1.APIServerSpec{
			TLSSecurityProfile: &configv1.TLSSecurityProfile{Type: configv1.TLSProfileModernType},
			TLSAdherence:       configv1.TLSAdherencePolicyNoOpinion,
		},
	}
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(obj).Build()
	watcher := newTLSProfileWatcher(fakeClient, *configv1.TLSProfiles[configv1.TLSProfileIntermediateType], cancel)
	request := ctrl.Request{NamespacedName: client.ObjectKey{Name: "cluster"}}
	_, err := watcher.Reconcile(t.Context(), request)
	require.NoError(t, err)
	require.NoError(t, ctx.Err(), "unhonored modern profile must not restart")
	require.NoError(t, fakeClient.Get(t.Context(), request.NamespacedName, obj))
	obj.Spec.TLSAdherence = configv1.TLSAdherencePolicyStrictAllComponents
	require.NoError(t, fakeClient.Update(t.Context(), obj))
	_, err = watcher.Reconcile(t.Context(), request)
	require.NoError(t, err)
	assert.ErrorIs(t, ctx.Err(), context.Canceled)
}

func TestAddonManagerRunnableLifecycle(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	frameworkContext := make(chan context.Context, 1)
	setup := make(chan struct{})
	var frameworkCtx context.Context
	runnable := &addonManagerRunnable{
		start: func(ctx context.Context) error {
			frameworkContext <- ctx
			return nil
		},
		setupControllers: func() error {
			select {
			case frameworkCtx = <-frameworkContext:
			default:
				return errors.New("controllers set up before the addon framework started")
			}
			close(setup)
			return nil
		},
	}
	assert.True(t, runnable.NeedLeaderElection())
	done := make(chan error, 1)
	go func() { done <- runnable.Start(ctx) }()
	select {
	case <-setup:
	case err := <-done:
		t.Fatalf("Start returned before controller setup: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("controller setup did not run")
	}
	require.NoError(t, frameworkCtx.Err())
	select {
	case err := <-done:
		t.Fatalf("Start returned before cancellation: %v", err)
	default:
	}
	cancel()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("Start did not stop on cancellation")
	}
	assert.ErrorIs(t, frameworkCtx.Err(), context.Canceled)
}

func TestAddonManagerRunnableErrors(t *testing.T) {
	for _, stage := range []string{"start", "setup"} {
		t.Run(stage, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			failure := errors.New(stage + " failed")
			var frameworkCtx context.Context
			var calls []string
			runnable := &addonManagerRunnable{
				start: func(ctx context.Context) error {
					frameworkCtx = ctx
					calls = append(calls, "start")
					if stage == "start" {
						return failure
					}
					return nil
				},
				setupControllers: func() error {
					calls = append(calls, "setup")
					return failure
				},
			}
			done := make(chan error, 1)
			go func() { done <- runnable.Start(ctx) }()
			select {
			case err := <-done:
				require.ErrorIs(t, err, failure)
			case <-time.After(5 * time.Second):
				t.Fatal("Start did not propagate the error")
			}
			if stage == "start" {
				assert.Equal(t, []string{"start"}, calls)
			} else {
				assert.Equal(t, []string{"start", "setup"}, calls)
			}
			require.NotNil(t, frameworkCtx)
			assert.ErrorIs(t, frameworkCtx.Err(), context.Canceled)
			assert.NoError(t, ctx.Err(), "only the framework child context should be canceled")
		})
	}
}

func TestManagerDynamicallyRegistersAddonControllers(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected API request: %s %s", r.Method, r.URL.Path)
		http.Error(w, "no API calls expected", http.StatusInternalServerError)
	}))
	defer api.Close()
	opts, err := newManagerOptions(*configv1.TLSProfiles[configv1.TLSProfileIntermediateType])
	require.NoError(t, err)
	opts.Metrics.BindAddress = "0"
	opts.HealthProbeBindAddress = "0"
	opts.Logger = logr.Discard()
	// An empty cache and static mapper keep this real manager independent of a cluster.
	opts.MapperProvider = func(*rest.Config, *http.Client) (meta.RESTMapper, error) {
		return meta.NewDefaultRESTMapper([]schema.GroupVersion{}), nil
	}
	mgr, err := ctrl.NewManager(&rest.Config{Host: api.URL}, opts)
	require.NoError(t, err)
	frameworkContext := make(chan context.Context, 1)
	controllerStarted := make(chan struct{})
	controllerStopped := make(chan struct{})
	var frameworkCtx context.Context
	require.NoError(t, mgr.Add(&addonManagerRunnable{
		start: func(ctx context.Context) error {
			frameworkContext <- ctx
			return nil
		},
		setupControllers: func() error {
			select {
			case frameworkCtx = <-frameworkContext:
			default:
				return errors.New("dynamic registration before the addon framework started")
			}
			return mgr.Add(manager.RunnableFunc(func(ctx context.Context) error {
				close(controllerStarted)
				<-ctx.Done()
				close(controllerStopped)
				return nil
			}))
		},
	}))
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- mgr.Start(ctx) }()
	select {
	case <-controllerStarted:
	case err := <-done:
		t.Fatalf("manager stopped before dynamic registration: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("dynamically registered controller did not start")
	}
	require.NoError(t, frameworkCtx.Err())
	cancel()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("manager did not shut down")
	}
	assert.ErrorIs(t, frameworkCtx.Err(), context.Canceled)
	select {
	case <-controllerStopped:
	default:
		t.Fatal("manager returned before its dynamic controller stopped")
	}
}
