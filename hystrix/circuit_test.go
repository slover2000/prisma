package hystrix

import (
	"time"
	"sync"
	"testing"
)

func TestGetCircuit(t *testing.T) {
	circuit1 := GetCircuit("foo")
	circuit2 := GetCircuit("foo")
	if circuit1 != circuit2 {
		t.Errorf("TestGetCircuit expect equals %v %v", circuit1, circuit2)
	}

	ConfigureCommand("command", CommandConfig{MaxQPS: 1000, RequestVolumeThreshold: 100, SleepWindow: 15, RollingWindows: 20, ErrorPercentThreshold: 30})
	circuitCommand := GetCircuit("command")
	if circuitCommand.name != "command" {
		t.Errorf("TestGetCircuit expect circuit name is %s", "command")
	}

	limit := circuitCommand.Limit()
	if float64(limit) != float64(1000) {
		t.Errorf("TestGetCircuit expect limit is %d", 1000)
	}

	if circuitCommand.Burst() != (1000 + 1) {
		t.Errorf("TestGetCircuit expect limit is %d", 1001)
	}
}

func BenchmarkGetCircuit(b *testing.B) {
	ConfigureCommand("command", CommandConfig{MaxQPS: 100, RequestVolumeThreshold: 50, SleepWindow: 5, RollingWindows: 10, ErrorPercentThreshold: 50})
	b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
			GetCircuit("command")
        }
    })
}

func TestCircuitToggleForeOpen(t *testing.T) {
	circuit := GetCircuit("foo")
	circuit.toggleForceOpen(true)
	if !circuit.IsOpen() {
		t.Errorf("TestGetCircuit expect circuit is not opened")
	}

	circuit.toggleForceOpen(false)
	if circuit.IsOpen() {
		t.Errorf("TestGetCircuit expect circuit is opened")
	}
}

func TestCircuitQPS(t *testing.T) {
	circuit := GetCircuit("foo1")
	config := getSettings("foo1")
	for i := 0; i < config.MaxQPS + 1; i++ {
		b := circuit.AllowRequest()
		if !b {
			t.Errorf("TestGetCircuit expect circuit allow request")
		}
	}

	for i := 0; i < 10; i++ {
		b := circuit.AllowRequest()
		if b {
			t.Errorf("TestGetCircuit expect circuit should not allowe request")
		}
	}

	time.Sleep(2 * time.Second)
	for i := 0; i < config.MaxQPS + 1; i++ {
		b := circuit.AllowRequest()
		if !b {
			t.Errorf("TestGetCircuit expect circuit allow request")
		}
	}	
}

func TestCircuitStatus(t *testing.T) {
	circuit := GetCircuit("foo2")
	config := getSettings("foo2")
	for i := 0; i < config.MaxQPS + 1; i++ {
		if circuit.AllowRequest() {
			circuit.ReportEvent(successTypeError)
		} else {
			t.Errorf("TestGetCircuit expect circuit allow request")
		}
	}
}

func TestCircuitErrorStatusWithFinalSucc(t *testing.T) {
	circuit := GetCircuit("foo3")
	config := getSettings("foo3")
	t.Parallel()
	var wg sync.WaitGroup
	wg.Add(config.RequestVolumeThreshold)
	for i := 0; i < config.RequestVolumeThreshold; i++ {
		go func() {
			if circuit.AttemptExecution() {
				circuit.ReportEvent(failureTypeError)
			}
			defer wg.Done()
		}()
	}
	wg.Wait()

	if circuit.AttemptExecution() {
		t.Errorf("TestGetCircuit expect circuit deny request")
	}

	time.Sleep(config.SleepWindow)
	var wg2 sync.WaitGroup
	wg2.Add(10)
	for i := 0; i < 10; i++ {
		go func() {
			if circuit.AttemptExecution() {
				circuit.ReportEvent(successTypeError)
			}
			defer wg2.Done()
		}()
	}
	wg2.Wait()
}

func TestCircuitErrorStatusWithFinalFailed(t *testing.T) {
	circuit := GetCircuit("foo4")
	config := getSettings("foo4")
	t.Parallel()
	var wg sync.WaitGroup
	wg.Add(config.RequestVolumeThreshold)	
	for i := 0; i < config.RequestVolumeThreshold; i++ {
		go func() {
			if circuit.AttemptExecution() {
				circuit.ReportEvent(failureTypeError)
			}
			defer wg.Done()			
		}()
	}
	wg.Wait()

	if circuit.AttemptExecution() {
		t.Errorf("TestGetCircuit expect circuit deny request")
	}

	time.Sleep(config.SleepWindow)
	var wg2 sync.WaitGroup
	wg2.Add(10)	
	for i := 0; i < 10; i++ {
		go func() {
			if circuit.AttemptExecution() {
				circuit.ReportEvent(timeoutTypeError)
			}
			defer wg2.Done()			
		}()	
	}
	wg2.Wait()
}