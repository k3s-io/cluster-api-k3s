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

import "time"

const (
	// deleteRequeueAfter is how long to wait before checking again to see if
	// all control plane machines have been deleted.
	deleteRequeueAfter = 30 * time.Second

	// preflightFailedRequeueAfter is how long to wait before trying to scale
	// up/down if some preflight check for those operation has failed.
	preflightFailedRequeueAfter = 15 * time.Second

	// dependentCertRequeueAfter is how long to wait before checking again to see if
	// dependent certificates have been created.
	dependentCertRequeueAfter = 30 * time.Second

	// etcdRemovalRequeueAfter is how long to wait before checking again to see if
	// etcd member is successfully removed.
	etcdRemovalRequeueAfter = 30 * time.Second

	k3sHookName = "k3s"

	kcpManagerName = "capi-kthreescontrolplane"
)
