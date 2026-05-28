package kubernetes

import "testing"

func TestNodeRowSchedulingState(t *testing.T) {
	ready := obj{
		"spec":   obj{"unschedulable": true},
		"status": obj{"conditions": []any{obj{"type": "Ready", "status": "True"}}},
	}
	row := nodeRow(ready)
	if row["unschedulable"] != true {
		t.Fatalf("cordoned node should expose unschedulable=true: %+v", row)
	}
	if row["status"] != "Ready,SchedulingDisabled" {
		t.Fatalf("cordoned status = %v, want Ready,SchedulingDisabled", row["status"])
	}

	schedulable := obj{"status": obj{"conditions": []any{obj{"type": "Ready", "status": "True"}}}}
	if r := nodeRow(schedulable); r["unschedulable"] != false || r["status"] != "Ready" {
		t.Fatalf("schedulable node row = %+v", r)
	}
}
