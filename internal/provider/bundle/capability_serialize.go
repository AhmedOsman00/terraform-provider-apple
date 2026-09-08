// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package bundle

import "sync"

// Apple applies a capability write as a read-modify-write over the parent
// Bundle ID's entire capability set, and does so without locking. Two creates
// issued concurrently against one Bundle ID therefore both answer 201 while
// only the later one survives: the winner's write is computed from a snapshot
// taken before the loser's had landed. Nothing in the response distinguishes
// this from success, so the capability simply goes missing, and the next plan
// proposes creating it again.
//
// Terraform applies independent resources in parallel, and two
// apple_bundle_id_capability resources that share a bundle_id are independent,
// so this is the default behaviour for the obvious configuration:
//
//	resource "apple_bundle_id_capability" "push"   { bundle_id = apple_bundle_id.app.id ... }
//	resource "apple_bundle_id_capability" "health" { bundle_id = apple_bundle_id.app.id ... }
//
// Serializing writes per Bundle ID inside the provider fixes it without asking
// every user to write depends_on between capabilities. The lock is held only
// for the duration of one API call, and only writes contend -- reads go through
// the parent's collection and are safe to run concurrently.
//
// This is per provider instance, which is the same scope as Terraform's
// parallelism: one terraform apply drives one instance of this provider. It
// does not defend against two applies running at once, which Terraform's state
// locking is responsible for.
var (
	bundleCapabilityLocks sync.Map // bundle ID -> *sync.Mutex

	// bundleCapabilityFallbackLock guards the unreachable case in which the map
	// holds something other than a mutex.
	bundleCapabilityFallbackLock sync.Mutex
)

// lockBundleCapabilities serializes capability writes for one Bundle ID and
// returns the function that releases the lock.
func lockBundleCapabilities(bundleID string) func() {
	value, _ := bundleCapabilityLocks.LoadOrStore(bundleID, &sync.Mutex{})

	mutex, ok := value.(*sync.Mutex)
	if !ok {
		// Nothing else writes to this map, so this cannot happen. Serializing
		// on a shared fallback is still safer than skipping the lock.
		mutex = &bundleCapabilityFallbackLock
	}

	mutex.Lock()

	return mutex.Unlock
}
