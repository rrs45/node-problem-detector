/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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

package types

import (
	"k8s.io/node-problem-detector/pkg/types"
	"time"
)

// Log is the log item returned by translator. It's very easy to extend this
// to support other log monitoring, such as docker log monitoring.
type Log struct {
	Timestamp time.Time
	Message   string
}

type SensuLog struct {
    Timestamp string `json:"timestamp"`
    Level     string `json:"level"`
    Message   string `json:"message"`
    Payload   struct {
        Client string `json:"client"`
        Check  struct {
            Command     string   `json:"command"`
            Contacts    []string `json:"contacts"`
            Handlers    []string `json:"handlers"`
            Info        string   `json:"info"`
            Occurrences int      `json:"occurrences"`
            Owner       string   `json:"owner"`
            Refresh     int      `json:"refresh"`
            Runbook     string   `json:"runbook"`
            Slack       string   `json:"slack"`
            Standalone  bool     `json:"standalone"`
            Timeout     int      `json:"timeout"`
            Name        string   `json:"name"`
            Issued      int      `json:"issued"`
            Executed    int      `json:"executed"`
            Duration    float64  `json:"duration"`
            Output      string   `json:"output"`
            Status      int      `json:"status"`
        } `json:"check"`
    } `json:"payload"`
}

// Rule describes how log monitor should analyze the log.
type Rule struct {
	// Type is the type of matched problem.
	Type types.Type `json:"type"`
	// Condition is the type of the condition the problem triggered. Notice that
	// the Condition field should be set only when the problem is permanent, or
	// else the field will be ignored.
	Condition string `json:"condition"`
	// Reason is the short reason of the problem.
	Reason string `json:"reason"`
	// Pattern is the regular expression to match the problem in log.
	// Notice that the pattern must match to the end of the line.
	Pattern string `json:"pattern"`
}
