/*


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

package controllers

import (
	"context"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	podconfigv1alpha1 "github.com/acmenezes/podconfig-operator/apis/podconfig/v1alpha1"
)

// TrunkPortReconciler reconciles a TrunkPort object
type TrunkPortReconciler struct {
	client.Client
	Log           logr.Logger
	Scheme        *runtime.Scheme
	trunkPortList *podconfigv1alpha1.TrunkPortList
}

// +kubebuilder:rbac:groups=podconfig.opdev.io,resources=trunkports,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=podconfig.opdev.io,resources=trunkports/status,verbs=get;update;patch

func (r *TrunkPortReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	_ = r.Log.WithValues("trunkport", req.NamespacedName)

	// update trunkport list

	// loop through the TrunkPortList

	// loop through the vlan list and apply configurations:

	// 1 - add functions add/remove vlans to the parent interface

	// 1b -  on the second iteration add other fields to CRD to hold ip configs
	// possible others vlan related as well
	// add functions apply/remove ip configurations

	// 2 - add vlan filtering to the Linux bridge carrying them out

	// 3 - on the third iteration validate configuration and update status

	return ctrl.Result{}, nil
}

func (r *TrunkPortReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&podconfigv1alpha1.TrunkPort{}).
		Complete(r)
}
