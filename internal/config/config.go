// Copyright 2025 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package config

import (
	"log"
	"time"

	"github.com/kelseyhightower/envconfig"
)

type Specification struct {
	// TODO: Mudar para 5 mins
	ControllerMinutesInternval time.Duration `default:"10s" envconfig:"controller_minutes_interval"`
}

var Spec Specification

func init() {
	err := envconfig.Process("", &Spec)
	if err != nil {
		log.Fatal(err.Error())
	}
}
