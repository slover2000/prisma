package hystrix

import (	
	"fmt"
	"time"
	"sync"	
	"testing"
	"golang.org/x/net/context"
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

	limit := circuitCommand.limiter.Limit()
	if float64(limit) != float64(1000) {
		t.Errorf("TestGetCircuit expect limit is %d", 1000)
	}

	if circuitCommand.limiter.Burst() != (1000 + 1) {
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
	ConfigureCommand("foo1", CommandConfig{MaxQPS: 100, RequestVolumeThreshold: 100, SleepWindow: 15, RollingWindows: 20, ErrorPercentThreshold: 30})
	circuit := GetCircuit("foo1")
	config := getSettings("foo1")
	t.Parallel()
	var wg sync.WaitGroup
	wg.Add(config.MaxQPS+1)
	for i := 0; i < config.MaxQPS + 1; i++ {
		go func() {
			b := circuit.AllowRequest()
			if !b {
				t.Errorf("TestGetCircuit expect circuit allow request")
			}
			wg.Done()
		}()
	}
	wg.Wait()

	wg.Add(10)
	for i := 0; i < 10; i++ {
		go func() {
			b := circuit.AllowRequest()
			if b {
				t.Errorf("TestGetCircuit expect circuit should not allowe request")
			}
			wg.Done()
		}()
	}
	wg.Wait()

	time.Sleep(2 * time.Second)
	wg.Add(config.MaxQPS+1)
	for i := 0; i < config.MaxQPS + 1; i++ {
		go func() {
			b := circuit.AllowRequest()
			if !b {
				t.Errorf("TestGetCircuit expect circuit allow request")
			}
			wg.Done()
		}()
	}
	wg.Wait()
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

func TestCircuitExecute(t *testing.T) {
	ConfigureCommand("command", CommandConfig{MaxConcurrent:5, MaxQPS: 1000, RequestVolumeThreshold: 100, SleepWindow: 15, RollingWindows: 20, ErrorPercentThreshold: 30})
	runFunc := func() (interface{}, error) {
		time.Sleep(10 * time.Millisecond)
		return "ok", nil
	}
	fbFunc := func(err error) (interface{}, error) {
		return nil, err
	}

	succ := 0
	failed := 0
	for i := 0; i < 100; i++ {
		time.Sleep(1 * time.Millisecond)
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(3) * time.Second)
			_, e := Execute(ctx, "command", runFunc, fbFunc)
			cancel()
			if e != nil {
				failed++
				fmt.Printf("TestCircuitExecute get error: %v\n", e)
			} else {
				succ++
			}
		}()
	}
	time.Sleep(10 * time.Second)
	stopAllCircuit(3)
	fmt.Printf("task succ[%d] failed[%d]\n", succ, failed)
}