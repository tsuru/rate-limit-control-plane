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
	ControllerMinutesInternal        time.Duration `default:"1s" envconfig:"controller_minutes_interval"`
	LogLevel                         string        `default:"info" envconfig:"log_level"`
	WarnZoneCollectionTime           time.Duration `default:"100ms" envconfig:"warn_zone_collection_time"`
	WarnZoneReadTime                 time.Duration `default:"50ms" envconfig:"warn_zone_read_time"`
	WarnZoneAggregationTime          time.Duration `default:"50ms" envconfig:"warn_zone_aggregation_time"`
	FeatureFlagPersistAggregatedData bool          `default:"false" envconfig:"feature_flag_persist_aggregated_data"`
	MaxTopOffendersReport            int           `default:"100" envconfig:"max_top_offenders_report"`
}

var Spec Specification

func init() {
	err := envconfig.Process("", &Spec)
	if err != nil {
		log.Fatal(err.Error())
	}
}
