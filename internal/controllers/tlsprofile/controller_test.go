// Copyright (c) Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project
// Licensed under the Apache License 2.0

package tlsprofile

import (
	"context"
	"testing"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	require.NoError(t, configv1.AddToScheme(s))
	return s
}

func TestReconcile(t *testing.T) {
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
					ObjectMeta: metav1.ObjectMeta{Name: apiServerName},
					Spec: configv1.APIServerSpec{
						TLSSecurityProfile: &configv1.TLSSecurityProfile{Type: tt.current},
						TLSAdherence:       tt.adherence,
					},
				}
				fakeClient := fake.NewClientBuilder().WithScheme(newTestScheme(t)).Build()
				r := NewReconciler(fakeClient, *configv1.TLSProfiles[tt.initial], cancel)
				request := ctrl.Request{NamespacedName: client.ObjectKey{Name: apiServerName}}
				if update {
					baseline := obj.DeepCopy()
					baseline.Spec.TLSSecurityProfile.Type = tt.initial
					baseline.Spec.TLSAdherence = configv1.TLSAdherencePolicyStrictAllComponents
					require.NoError(t, fakeClient.Create(t.Context(), baseline))
					_, err := r.Reconcile(t.Context(), request)
					require.NoError(t, err)
					require.NoError(t, ctx.Err(), "baseline must not restart")
					obj.ResourceVersion = baseline.ResourceVersion
					require.NoError(t, fakeClient.Update(t.Context(), obj))
				} else {
					require.NoError(t, fakeClient.Create(t.Context(), obj))
				}
				_, err := r.Reconcile(t.Context(), request)
				require.NoError(t, err)
				assert.Equal(t, tt.wantRestart, ctx.Err() != nil, "update=%t", update)
			}
		})
	}
}

func TestReconcileAdherenceOnlyChange(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	obj := &configv1.APIServer{
		ObjectMeta: metav1.ObjectMeta{Name: apiServerName},
		Spec: configv1.APIServerSpec{
			TLSSecurityProfile: &configv1.TLSSecurityProfile{Type: configv1.TLSProfileModernType},
			TLSAdherence:       configv1.TLSAdherencePolicyNoOpinion,
		},
	}
	fakeClient := fake.NewClientBuilder().WithScheme(newTestScheme(t)).WithObjects(obj).Build()
	r := NewReconciler(fakeClient, *configv1.TLSProfiles[configv1.TLSProfileIntermediateType], cancel)
	request := ctrl.Request{NamespacedName: client.ObjectKey{Name: apiServerName}}
	_, err := r.Reconcile(t.Context(), request)
	require.NoError(t, err)
	require.NoError(t, ctx.Err(), "unhonored modern profile must not restart")
	require.NoError(t, fakeClient.Get(t.Context(), request.NamespacedName, obj))
	obj.Spec.TLSAdherence = configv1.TLSAdherencePolicyStrictAllComponents
	require.NoError(t, fakeClient.Update(t.Context(), obj))
	_, err = r.Reconcile(t.Context(), request)
	require.NoError(t, err)
	assert.ErrorIs(t, ctx.Err(), context.Canceled)
}
