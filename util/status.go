package util

import "sync"

//as of now only one task can run at a time

type ActionStatus struct {
	sync.Mutex
	IsActive   bool   `json:"isactive"`
	Session    string `json:"session"`
	Err        error  `json:"err"`
	ActionType string `json:"tasktype"`
	Progress   string `json:"progress"`
}

var actionUpdate = ActionStatus{}

func GetActionStatus() *ActionStatus {
	return &actionUpdate
}

func StartAction(atype string, session string) {
	actionUpdate.Lock()
	defer actionUpdate.Unlock()
	actionUpdate.IsActive = true
	actionUpdate.Session = session
	actionUpdate.Err = nil
	actionUpdate.ActionType = atype
}
func ErrorAction(err error) {
	actionUpdate.Lock()
	defer actionUpdate.Unlock()
	actionUpdate.Err = err
	actionUpdate.IsActive = false
}
func UpdateActionProgress(progress string) {
	actionUpdate.Lock()
	defer actionUpdate.Unlock()
	actionUpdate.Progress = progress
}
func CloseAction() {
	actionUpdate.Lock()
	defer actionUpdate.Unlock()
	actionUpdate.IsActive = false
}

func IsActionRunning() bool {
	return actionUpdate.IsActive
}
