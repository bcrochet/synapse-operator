/*
Copyright 2021.

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

package synapse

import (
	"context"
	"errors"
	"reflect"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	pgov1beta1 "github.com/crunchydata/postgres-operator/pkg/apis/postgres-operator.crunchydata.com/v1beta1"
	subreconciler "github.com/opdev/subreconciler"
	synapsev1alpha1 "github.com/opdev/synapse-operator/apis/synapse/v1alpha1"
)

// SynapseReconciler reconciles a Synapse object
type SynapseReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

type HomeserverPgsqlDatabase struct {
	Name     string `yaml:"name"`
	TxnLimit int64  `yaml:"txn_limit"`
	Args     struct {
		User     string `yaml:"user"`
		Password string `yaml:"password"`
		Database string `yaml:"database"`
		Host     string `yaml:"host"`
		Port     int64  `yaml:"port"`
		CpMin    int64  `yaml:"cp_min"`
		CpMax    int64  `yaml:"cp_max"`
	}
}

//+kubebuilder:rbac:groups=synapse.opdev.io,resources=synapses,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=synapse.opdev.io,resources=synapses/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=synapse.opdev.io,resources=synapses/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=services;persistentvolumeclaims;configmaps;serviceaccounts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=postgres-operator.crunchydata.com,resources=postgresclusters,verbs=get;list;watch;create;update;patch;delete

func GetPostgresClusterResourceName(synapse synapsev1alpha1.Synapse) string {
	return strings.Join([]string{synapse.Name, "pgsql"}, "-")
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.9.2/pkg/reconcile
func (r *SynapseReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)

	var synapse synapsev1alpha1.Synapse // The Synapse object being reconciled
	if r, err := r.getLatestSynapse(ctx, req, &synapse); subreconciler.ShouldHaltOrRequeue(r, err) {
		return subreconciler.Evaluate(r, err)
	}

	// The list of subreconcilers for Synapse.
	var subreconcilersForSynapse []subreconciler.FnWithRequest

	// Synapse should either have a Spec.Homeserver.ConfigMap or Spec.Homeserver.Values
	if synapse.Spec.Homeserver.ConfigMap != nil {
		// If the user provided a ConfigMap for the Homeserver config file:
		// * We ensure that it exists and is a valid yaml file
		// * We populate the Status.HomeserverConfiguration with the values defined in the input ConfigMap
		// * We create a copy of the user-provided ConfigMap.
		subreconcilersForSynapse = []subreconciler.FnWithRequest{
			r.parseInputSynapseConfigMap,
			r.copyInputSynapseConfigMap,
		}
	} else {
		// If the user hasn't provided a ConfigMap with a custom
		// homeserver.yaml, we create a new ConfigMap. The default
		// homeserver.yaml is configured with values defined in
		// Spec.Homeserver.Values
		subreconcilersForSynapse = []subreconciler.FnWithRequest{
			r.setStatusHomeserverConfiguration,
			r.reconcileSynapseConfigMap,
		}
	}

	// Determine the existence of Bridges referencing this Synapse instance
	subreconcilersForSynapse = append(
		subreconcilersForSynapse,
		r.updateSynapseStatusBridges,
	)

	if synapse.Spec.CreateNewPostgreSQL {
		if !r.isPostgresOperatorInstalled(ctx) {
			reason := "Cannot create PostgreSQL instance for synapse. Postgres-operator is not installed."
			if err := r.setFailedState(ctx, &synapse, reason); err != nil {
				log.Error(err, "Error updating Synapse State")
			}

			err := errors.New("cannot create PostgreSQL instance for synapse. Potsgres-operator is not installed")
			log.Error(err, "Cannot create PostgreSQL instance for synapse. Potsgres-operator is not installed.")
			return subreconciler.Evaluate(subreconciler.DoNotRequeue())
		}

		// Reconcile the PostgresCluster CR and ConfigMap.
		// Also update the Synapse Status and ConfigMap with database
		// connection information.
		subreconcilersForSynapse = append(
			subreconcilersForSynapse,
			r.reconcilePostgresClusterConfigMap,
			r.reconcilePostgresClusterCR,
			r.updateSynapseStatusWithPostgreSQLInfos,
			r.updateSynapseConfigMapForPostgresCluster,
		)
	}

	if synapse.Status.Bridges.Heisenbridge.Enabled {
		// Add the update of the Synapse ConfigMap to the Synapse
		// subreconciler list. This is to prepare for future work. When using
		// a multi API approach, we forsee this task to be performed by the
		// Synapse controller (as opposed to the Heisenbridge controller,
		// performing all task listed in subreconcilersForHeisenbridge).
		subreconcilersForSynapse = append(subreconcilersForSynapse, r.updateSynapseConfigMapForHeisenbridge)
	}

	if synapse.Status.Bridges.MautrixSignal.Enabled {
		// Add the update of the Synapse ConfigMap to the Synapse
		// subreconciler list. This is to prepare for future work. When using
		// a multi API approach, we forsee this task to be performed by the
		// Synapse controller (as opposed to the mautrix-signal controller,
		// performing all task listed in subreconcilersForMautrixSignal).
		subreconcilersForSynapse = append(subreconcilersForSynapse, r.updateSynapseConfigMapForMautrixSignal)
	}

	// SA and RB are only necessary if we're running on OpenShift
	if synapse.Spec.IsOpenshift {
		subreconcilersForSynapse = append(
			subreconcilersForSynapse,
			r.reconcileSynapseServiceAccount,
			r.reconcileSynapseRoleBinding,
		)
	}

	// Reconcile Synapse resources: Service, PVC, Deployment
	subreconcilersForSynapse = append(
		subreconcilersForSynapse,
		r.reconcileSynapseService,
		r.reconcileSynapsePVC,
		r.reconcileSynapseDeployment,
		r.setSynapseStatusAsRunning,
	)

	// Run all subreconcilers sequentially
	for _, f := range subreconcilersForSynapse {
		if r, err := f(ctx, req); subreconciler.ShouldHaltOrRequeue(r, err) {
			return subreconciler.Evaluate(r, err)
		}
	}

	return subreconciler.Evaluate(subreconciler.DoNotRequeue())
}

func (r *SynapseReconciler) getLatestSynapse(
	ctx context.Context,
	req ctrl.Request,
	s *synapsev1alpha1.Synapse,
) (*ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)

	if err := r.Get(ctx, req.NamespacedName, s); err != nil {
		if k8serrors.IsNotFound(err) {
			// we'll ignore not-found errors, since they can't be fixed by an immediate
			// requeue (we'll need to wait for a new notification), and we can get them
			// on deleted requests.
			log.Error(
				err,
				"Cannot find Synapse - has it been deleted ?",
				"Synapse Name", s.Name,
				"Synapse Namespace", s.Namespace,
			)
			return subreconciler.DoNotRequeue()
		}
		log.Error(
			err,
			"Error fetching Synapse",
			"Synapse Name", s.Name,
			"Synapse Namespace", s.Namespace,
		)
		return subreconciler.RequeueWithError(err)
	}

	return subreconciler.ContinueReconciling()
}

// labelsForSynapse returns the labels for selecting the resources
// belonging to the given synapse CR name.
func labelsForSynapse(name string) map[string]string {
	return map[string]string{"app": "synapse", "synapse_cr": name}
}

func (r *SynapseReconciler) setFailedState(ctx context.Context, synapse *synapsev1alpha1.Synapse, reason string) error {
	synapse.Status.State = "FAILED"
	synapse.Status.Reason = reason

	err, _ := r.updateSynapseStatus(ctx, synapse)
	return err
}

func (r *SynapseReconciler) updateSynapseStatus(ctx context.Context, synapse *synapsev1alpha1.Synapse) (error, bool) {
	current := &synapsev1alpha1.Synapse{}
	if err := r.Get(
		ctx,
		types.NamespacedName{Name: synapse.Name, Namespace: synapse.Namespace},
		current,
	); err != nil {
		return err, false
	}

	if !reflect.DeepEqual(synapse.Status, current.Status) {
		if err := r.Status().Patch(ctx, synapse, client.MergeFrom(current)); err != nil {
			return err, false
		}
		return nil, true
	}

	return nil, false
}

func (r *SynapseReconciler) setStatusHomeserverConfiguration(ctx context.Context, req ctrl.Request) (*ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)

	s := &synapsev1alpha1.Synapse{}
	if r, err := r.getLatestSynapse(ctx, req, s); subreconciler.ShouldHaltOrRequeue(r, err) {
		return r, err
	}

	s.Status.HomeserverConfiguration.ServerName = s.Spec.Homeserver.Values.ServerName
	s.Status.HomeserverConfiguration.ReportStats = s.Spec.Homeserver.Values.ReportStats

	err, has_patched := r.updateSynapseStatus(ctx, s)
	if err != nil {
		log.Error(err, "Error updating Synapse Status")
		return subreconciler.RequeueWithError(err)
	}
	if has_patched {
		return subreconciler.Requeue()
	}

	return subreconciler.ContinueReconciling()
}

func (r *SynapseReconciler) isPostgresOperatorInstalled(ctx context.Context) bool {
	err := r.Client.List(ctx, &pgov1beta1.PostgresClusterList{})
	return err == nil
}

// updateSynapseStatusWithPostgreSQLInfos is a function of type
// FnWithRequest, to be called in the main reconciliation loop.
//
// It parses the PostgresCluster Secret and updates the Synapse status with the
// database connection information.
func (r *SynapseReconciler) updateSynapseStatusWithPostgreSQLInfos(ctx context.Context, req ctrl.Request) (*ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)

	s := &synapsev1alpha1.Synapse{}
	if r, err := r.getLatestSynapse(ctx, req, s); subreconciler.ShouldHaltOrRequeue(r, err) {
		return r, err
	}

	var postgresSecret corev1.Secret

	keyForPostgresClusterSecret := types.NamespacedName{
		Name:      GetPostgresClusterResourceName(*s) + "-pguser-synapse",
		Namespace: s.Namespace,
	}

	// Get PostgresCluster Secret containing information for the synapse user
	if err := r.Get(ctx, keyForPostgresClusterSecret, &postgresSecret); err != nil {
		return subreconciler.RequeueWithError(err)
	}

	// Locally updates the Synapse Status
	if err := r.updateSynapseStatusDatabase(s, postgresSecret); err != nil {
		return subreconciler.RequeueWithError(err)
	}

	// Actually sends an API request to update the Status
	err, has_patched := r.updateSynapseStatus(ctx, s)
	if err != nil {
		log.Error(err, "Error updating Synapse Status")
		return subreconciler.RequeueWithError(err)
	}
	if has_patched {
		return subreconciler.Requeue()
	}

	return subreconciler.ContinueReconciling()
}

func (r *SynapseReconciler) updateSynapseStatusDatabase(
	s *synapsev1alpha1.Synapse,
	postgresSecret corev1.Secret,
) error {
	var postgresSecretData map[string][]byte = postgresSecret.Data

	host, ok := postgresSecretData["host"]
	if !ok {
		err := errors.New("missing host in PostgreSQL Secret")
		// log.Error(err, "Missing host in PostgreSQL Secret")
		return err
	}

	port, ok := postgresSecretData["port"]
	if !ok {
		err := errors.New("missing port in PostgreSQL Secret")
		// log.Error(err, "Missing port in PostgreSQL Secret")
		return err
	}

	// See https://github.com/opdev/synapse-operator/issues/12
	// databaseName, ok := postgresSecretData["dbname"]
	_, ok = postgresSecretData["dbname"]
	if !ok {
		err := errors.New("missing dbname in PostgreSQL Secret")
		// log.Error(err, "Missing dbname in PostgreSQL Secret")
		return err
	}

	user, ok := postgresSecretData["user"]
	if !ok {
		err := errors.New("missing user in PostgreSQL Secret")
		// log.Error(err, "Missing user in PostgreSQL Secret")
		return err
	}

	password, ok := postgresSecretData["password"]
	if !ok {
		err := errors.New("missing password in PostgreSQL Secret")
		// log.Error(err, "Missing password in PostgreSQL Secret")
		return err
	}

	s.Status.DatabaseConnectionInfo.ConnectionURL = string(host) + ":" + string(port)
	// s.Status.DatabaseConnectionInfo.DatabaseName = string(databaseName) // See https://github.com/opdev/synapse-operator/issues/12
	s.Status.DatabaseConnectionInfo.DatabaseName = "synapse"
	s.Status.DatabaseConnectionInfo.User = string(user)
	s.Status.DatabaseConnectionInfo.Password = string(base64encode(string(password)))
	s.Status.DatabaseConnectionInfo.State = "READY"

	return nil
}

// setSynapseStatusAsRunning is a function of type FnWithRequest, to be
// called in the main reconciliation loop.
//
// It set the Synapse Status 'State' field to 'RUNNING'.
func (r *SynapseReconciler) setSynapseStatusAsRunning(ctx context.Context, req ctrl.Request) (*ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)

	s := &synapsev1alpha1.Synapse{}
	if r, err := r.getLatestSynapse(ctx, req, s); subreconciler.ShouldHaltOrRequeue(r, err) {
		return r, err
	}

	s.Status.NeedsReconcile = false
	s.Status.State = "RUNNING"
	s.Status.Reason = ""

	err, has_patched := r.updateSynapseStatus(ctx, s)
	if err != nil {
		log.Error(err, "Error updating Synapse Status")
		return subreconciler.RequeueWithError(err)
	}
	if has_patched {
		return subreconciler.Requeue()
	}

	return subreconciler.ContinueReconciling()
}

func (r *SynapseReconciler) updateSynapseStatusBridges(ctx context.Context, req ctrl.Request) (*ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)

	s := &synapsev1alpha1.Synapse{}
	if r, err := r.getLatestSynapse(ctx, req, s); subreconciler.ShouldHaltOrRequeue(r, err) {
		return r, err
	}

	hList := &synapsev1alpha1.HeisenbridgeList{}

	r.Client.List(ctx, hList)
	for _, h := range hList.Items {
		if h.Spec.Synapse.Name == s.Name {
			s.Status.Bridges.Heisenbridge.Enabled = true
			s.Status.Bridges.Heisenbridge.Name = h.Name
		}
	}

	msList := &synapsev1alpha1.MautrixSignalList{}
	r.Client.List(ctx, msList)
	for _, ms := range msList.Items {
		if ms.Spec.Synapse.Name == s.Name {
			s.Status.Bridges.MautrixSignal.Enabled = true
			s.Status.Bridges.MautrixSignal.Name = ms.Name
		}
	}

	err, has_patched := r.updateSynapseStatus(ctx, s)
	if err != nil {
		log.Error(err, "Error updating Synapse Status")
		return subreconciler.RequeueWithError(err)
	}
	if has_patched {
		return subreconciler.Requeue()
	}

	return subreconciler.ContinueReconciling()
}

// SetupWithManager sets up the controller with the Manager.
func (r *SynapseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&synapsev1alpha1.Synapse{}).
		Owns(&corev1.Service{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Complete(r)
}
