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

package systemlogmonitor

import (
	"encoding/json"
	"io/ioutil"
	"time"
	"fmt"
	"github.com/golang/glog"
	"regexp"

	"k8s.io/node-problem-detector/pkg/systemlogmonitor/logwatchers"
	watchertypes "k8s.io/node-problem-detector/pkg/systemlogmonitor/logwatchers/types"
	logtypes "k8s.io/node-problem-detector/pkg/systemlogmonitor/types"
	systemlogtypes "k8s.io/node-problem-detector/pkg/systemlogmonitor/types"
	"k8s.io/node-problem-detector/pkg/types"
	"k8s.io/node-problem-detector/pkg/util"
	"k8s.io/node-problem-detector/pkg/util/tomb"
)

type logMonitor struct {
	watcher    watchertypes.LogWatcher
	buffer     LogBuffer
	config     MonitorConfig
	conditions []types.Condition
	logCh      <-chan *logtypes.Log
	output     chan *types.Status
	tomb       *tomb.Tomb
}

type SensulogMonitor struct {
	watcher    watchertypes.SensuLogWatcher
	buffer     LogBuffer
	config     MonitorConfig
	conditions []types.Condition
	logCh      <-chan *logtypes.SensuLog
	output     chan *types.Status
	tomb       *tomb.Tomb
}

type check_store struct {
	timestamp time.Time
	check string
	output string
	level  string
}
var checks_status_arr =  []check_store{}

// NewLogMonitorOrDie create a new LogMonitor, panic if error occurs.
func NewSensuLogMonitorOrDie(configPath string) types.Monitor {
	s := &SensulogMonitor{
		tomb: tomb.NewTomb(),
	}
	f, err := ioutil.ReadFile(configPath)
	if err != nil {
		glog.Fatalf("Failed to read configuration file %q: %v", configPath, err)
	}
	err = json.Unmarshal(f, &s.config)
	if err != nil {
		glog.Fatalf("Failed to unmarshal configuration file %q: %v", configPath, err)
	}
		
	// Apply default configurations
	(&s.config).ApplyDefaultConfiguration()
	err = s.config.ValidateRules()
	if err != nil {
		glog.Fatalf("Failed to validate matching rules %+v: %v", s.config.Rules, err)
	}
	glog.Infof("Finish parsing log monitor config file: %+v", s.config)
	s.watcher = logwatchers.GetSensuLogWatcherOrDie(s.config.WatcherConfig)
	s.buffer = NewLogBuffer(s.config.BufferSize)
	// A 1000 size channel should be big enough.
	s.output = make(chan *types.Status, 1000)
	return s
}

func NewLogMonitorOrDie(configPath string) types.Monitor {
	l := &logMonitor{
		tomb: tomb.NewTomb(),
	}
	f, err := ioutil.ReadFile(configPath)
	if err != nil {
		glog.Fatalf("Failed to read configuration file %q: %v", configPath, err)
	}
	err = json.Unmarshal(f, &l.config)
	if err != nil {
		glog.Fatalf("Failed to unmarshal configuration file %q: %v", configPath, err)
	}
		
	// Apply default configurations
	(&l.config).ApplyDefaultConfiguration()
	err = l.config.ValidateRules()
	if err != nil {
		glog.Fatalf("Failed to validate matching rules %+v: %v", l.config.Rules, err)
	}
	glog.Infof("Finish parsing log monitor config file: %+v", l.config)
	l.watcher = logwatchers.GetLogWatcherOrDie(l.config.WatcherConfig)
	l.buffer = NewLogBuffer(l.config.BufferSize)
	// A 1000 size channel should be big enough.
	l.output = make(chan *types.Status, 1000)
	return l
}

func (l *logMonitor) Start() (<-chan *types.Status, error) {
	glog.Info("Start log monitor")
	var err error
	l.logCh, err = l.watcher.Watch()
	if err != nil {
		return nil, err
	}
	go l.monitorLoop()
	return l.output, nil
}

func (l *logMonitor) Stop() {
	glog.Info("Stop log monitor")
	l.tomb.Stop()
}

func (s *SensulogMonitor) Stop() {
	glog.Info("Stop log monitor")
	s.tomb.Stop()
}

func (s *SensulogMonitor) Start() (<-chan *types.Status, error) {
	glog.Info("Start log monitor")
	var err error
	s.logCh, err = s.watcher.Watch()
	if err != nil {
		return nil, err
	}
	go s.monitorLoop()
	return s.output, nil
}

// monitorLoop is the main loop of log monitor.
func (s *SensulogMonitor) monitorLoop() {
	defer s.tomb.Done()
	s.initializeStatus()
	for {
		select {
		case log := <-s.logCh:
			s.parseLog(log)
		case <-s.tomb.Stopping():
			s.watcher.Stop()
			glog.Infof("Log monitor stopped")
			return
		}
	}
}

// monitorLoop is the main loop of log monitor.
func (l *logMonitor) monitorLoop() {
	defer l.tomb.Done()
	l.initializeStatus()
	for {
		select {
		case log := <-l.logCh:
			l.parseLog(log)
		case <-l.tomb.Stopping():
			l.watcher.Stop()
			glog.Infof("Log monitor stopped")
			return
		}
	}
}

// parseLog parses one log line.
//EDIT: here we need to match config file for sensu
func (s *SensulogMonitor) parseLog(log *logtypes.SensuLog) {
	// Once there is new log, log monitor will push it into the log buffer and try
	// to match each rule. If any rule is matched, log monitor will report a status.
	//fmt.Println("log monitor got new log: %+v", log)
	// buffer: add check if new, check with old state , return changed
	// check_map[check] = output, if old is ok & new is crit => condition is true, add check to cond, else old is crit & new is ok
	// remove check from cond . if len(check)> 0 generate status, else "all passed"

	crit_matched, _ := regexp.MatchString("CRITICAL", log.Output )
	warn_matched, _ := regexp.MatchString("WARN", log.Output )
	ok_matched, _   := regexp.MatchString("OK", log.Output ) 
	
	b := checks_status_arr[:0]

	update := false
	new_elem := true
	for i, elem := range checks_status_arr {
		//If previously present
		if elem.check == log.Check {
			new_elem = false
			if crit_matched { 
				elem.level = "CRITICAL"
				elem.timestamp = log.Timestamp
		
			} else if warn_matched{
				elem.level = "WARN"
				elem.timestamp = log.Timestamp
			} else if ok_matched{
				//delete element if ok
				b = append(checks_status_arr[:i], checks_status_arr[i+1:]...)
				update = true	
				
			}
		}
	}
			
	if crit_matched && new_elem {
		checks_status_arr = append(checks_status_arr, check_store{log.Timestamp, log.Check, log.Output, "CRITICAL"})
	} else if warn_matched && new_elem {		   
		checks_status_arr = append(checks_status_arr, check_store{log.Timestamp, log.Check, log.Output, "WARN"})
	}
				
	
	if update {
		status := s.generateSensuStatus(b)
		s.output <- status
	} else {
		status := s.generateSensuStatus(checks_status_arr)
		s.output <- status
	}
	
}


// parseLog parses one log line.
func (l *logMonitor) parseLog(log *logtypes.Log) {
	// Once there is new log, log monitor will push it into the log buffer and try
	// to match each rule. If any rule is matched, log monitor will report a status.
	l.buffer.Push(log)
	for _, rule := range l.config.Rules {
		matched := l.buffer.Match(rule.Pattern)
		if len(matched) == 0 {
			continue
		}
		status := l.generateStatus(matched, rule)
		glog.Infof("New status generated: %+v", status)
		l.output <- status
	}
}


// generateSensuStatus generates status from the logs.
func (s *SensulogMonitor) generateSensuStatus(logs_arr []check_store) *types.Status {
	// We use the timestamp of the first log line as the timestamp of the status.
	
	//message := generateMessage(logs_arr)
	

	var events []types.Event
	if len(logs_arr) > 0 {
		for _, elem := range logs_arr {
			events = append(events, util.GenerateSensuConditionChangeEvent(
						elem.check,
						"SensuCheckFailed",
						elem.timestamp,
			))
		}
	} 
	
	// For permanent error changes the condition
	for i := range s.conditions {
		condition := &s.conditions[i]
		
		// Update transition timestamp and message when the condition
		// changes. Condition is considered to be changed only when
		// status or reason changes.
	
		if len(logs_arr) == 0 {
			condition.Transition = time.Now()
			condition.Status = types.False
			condition.Reason = "AllChecksFailed"
			condition.Message = "All Sensu Checks passed"
						
		} else {
			out, err := json.Marshal(logs_arr)
    			if err != nil {
				panic (fmt.Sprintf("log_monitor: cannot unmarshal struct %v",err))
    				}
			condition.Transition = time.Now()
			condition.Message = string(out)
			condition.Status = types.True
			condition.Reason = "SomeChecksFailed"
			}
	}
	return &types.Status{
		Source: "Sensu",
		// TODO(random-liu): Aggregate events and conditions and then do periodically report.
		Events:     events,
		Conditions: s.conditions,
	}
}


// generateStatus generates status from the logs.
func (l *logMonitor) generateStatus(logs []*logtypes.Log, rule systemlogtypes.Rule) *types.Status {
	// We use the timestamp of the first log line as the timestamp of the status.
	timestamp := logs[0].Timestamp
	message := generateMessage(logs)
	var events []types.Event

	if rule.Type == types.Temp {
		// For temporary error only generate event
		events = append(events, types.Event{
			Severity:  types.Warn,
			Timestamp: timestamp,
			Reason:    rule.Reason,
			Message:   message,
		})
	} else {
		// For permanent error changes the condition
		for i := range l.conditions {
			condition := &l.conditions[i]
			if condition.Type == rule.Condition {
				// Update transition timestamp and message when the condition
				// changes. Condition is considered to be changed only when
				// status or reason changes.
				if condition.Status == types.False || condition.Reason != rule.Reason {
					condition.Transition = timestamp
					condition.Message = message
					events = append(events, util.GenerateConditionChangeEvent(
						condition.Type,
						types.True,
						rule.Reason,
						timestamp,
					))
				}
				condition.Status = types.True
				condition.Reason = rule.Reason
				break
			}
		}
	}
	return &types.Status{
		Source: l.config.Source,
		// TODO(random-liu): Aggregate events and conditions and then do periodically report.
		Events:     events,
		Conditions: l.conditions,
	}
}

// initializeStatus initializes the internal condition and also reports it to the node problem detector.
func (s *SensulogMonitor) initializeStatus() {
	// Initialize the default node conditions
	s.conditions = initialConditions(s.config.DefaultConditions)
	glog.Infof("Initialize condition generated: %+v", s.conditions)
	// Update the initial status
	s.output <- &types.Status{
		Source:     s.config.Source,
		Conditions: s.conditions,
	}
}

func (l *logMonitor) initializeStatus() {
	// Initialize the default node conditions
	l.conditions = initialConditions(l.config.DefaultConditions)
	glog.Infof("Initialize condition generated: %+v", l.conditions)
	// Update the initial status
	l.output <- &types.Status{
		Source:     l.config.Source,
		Conditions: l.conditions,
	}
}


func initialConditions(defaults []types.Condition) []types.Condition {
	conditions := make([]types.Condition, len(defaults))
	copy(conditions, defaults)
	for i := range conditions {
		conditions[i].Status = types.False
		conditions[i].Transition = time.Now()
	}
	return conditions
}

func generateMessage(logs []*logtypes.Log) string {
	messages := []string{}
	for _, log := range logs {
		messages = append(messages, log.Message)
	}
	return concatLogs(messages)
}
