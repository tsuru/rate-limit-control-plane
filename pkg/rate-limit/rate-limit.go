// Copyright 2025 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ratelimit

// TODO: Make a comment noting that this references the fields from nginx rate limit typing
type RateLimitEntry struct {
	Key    []byte
	Last   int64
	Excess int64
}

type RateLimitPodZone struct {
	PodIP            string
	Zone             string
	RateLimitEntries []RateLimitEntry
}
