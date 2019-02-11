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
package sensulog

import (
	"fmt"
	"regexp"
	"time"
	"encoding/json"
	"strings"

	logtypes "k8s.io/node-problem-detector/pkg/systemlogmonitor/types"

	"github.com/golang/glog"
)

// translator translates sensu log line into internal log type based on user defined
// translate line and return logtypes.SensuLog
type SensuJsonLog struct {
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

type translator struct {
	sensuchecks string
	timestampFormat string
}

func newTranslatorOrDie(pluginConfig map[string]string) *translator {
	if err := validatePluginConfig(pluginConfig); err != nil {
		glog.Errorf("Failed to validate plugin configuration %+v: %v", pluginConfig, err)
	}
	
	return &translator{
		sensuchecks: pluginConfig[checks],
		timestampFormat: pluginConfig[timestampFormatKey],
	}
}

// translate translates the log line into internal type.
func (t *translator) translate(line string) (*logtypes.Log, error) {
	// Unmarshal Json line
	var sensulog SensuJsonLog
	byt := []byte(line)
	err := json.Unmarshal(byt, &sensulog)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal line ")
	} else {
		glog.Infof("Successfully unmarshaled line %q", line)
	}
	
	// Parse timestamp.
	timestamp, err := time.ParseInLocation(t.timestampFormat, sensulog.Timestamp, time.Local)
	if err != nil {
		return nil, fmt.Errorf("failed to parse timestamp %q: %v", matches[len(matches)-1], err)
	}
	messsage := []string
	// Loop through all checks and compare
	for c := range strings.Split(sensuchecks, ",") {
		if sensulog.Paylod.Check.Name == c {
			//need to apped all matched checks
			message := append(message, "[" + c ">>" + sensulog.Paylod.Check.Output + "]")
		}
	}
	
	return &logtypes.Log{
		Timestamp: timestamp,
		Message:   message,
	}, nil
}

func validatePluginConfig(cfg map[string]string) error {
	if cfg[timestampKey] == "" {
		return fmt.Errorf("unexpected empty timestamp regular expression")
	}
	if cfg[checks] == "" {
		return fmt.Errorf("unexpected empty checks")
	}
	
	return nil
}
