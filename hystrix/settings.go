package hystrix

import (
	"sync"
	"time"	
)

const (
	// DefaultMaxQPS is how many commands of the same type can run at the same time
	DefaultMaxQPS = 100

	// DefaultVolumeThreshold is the minimum number of requests needed before a circuit can be tripped due to health
	DefaultVolumeThreshold = 20

	// DefaultSleepWindow is how long, in milliseconds, to wait after a circuit opens before testing for recovery
	DefaultSleepWindow = 5000

	// DefaultErrorPercentThreshold causes circuits to open once the rolling measure of errors exceeds this percent of requests
	DefaultErrorPercentThreshold = 50

	// DefaultRollingWindows reserved define how many values should be reserved in seconds
	DefaultRollingWindows = int((DefaultSleepWindow / 1000) * 100 / DefaultErrorPercentThreshold)
)

// Settings hystrix settings
type Settings struct {	
	MaxQPS  			   int
	RequestVolumeThreshold int
	SleepWindow            time.Duration
	ErrorPercentThreshold  int
	RollingWindows         int
}

// CommandConfig is used to tune circuit settings at runtime
type CommandConfig struct {	
	MaxQPS  			   int `json:"max_qps"`
	RequestVolumeThreshold int `json:"request_volume_threshold"`
	SleepWindow            int `json:"sleep_window"`
	ErrorPercentThreshold  int `json:"error_percent_threshold"`
	RollingWindows         int `json:"rolling_windows"`
}

var circuitSettings *sync.Map

func init() {
	circuitSettings = &sync.Map{}
}

// ConfigureCommand applies settings for a circuit
func ConfigureCommand(name string, config CommandConfig) {
	qps := DefaultMaxQPS
	if config.MaxQPS != 0 {
		qps = config.MaxQPS
	}

	volume := DefaultVolumeThreshold
	if config.RequestVolumeThreshold != 0 {
		volume = config.RequestVolumeThreshold
	}

	sleep := DefaultSleepWindow
	if config.SleepWindow != 0 {
		sleep = config.SleepWindow
	}

	errorPercent := DefaultErrorPercentThreshold
	if config.ErrorPercentThreshold != 0 {
		errorPercent = config.ErrorPercentThreshold
	}

	rollingWindows := DefaultRollingWindows
	if config.RollingWindows != 0 {
		rollingWindows = config.RollingWindows
	}	

	setting := &Settings{
		MaxQPS:                 qps,
		RequestVolumeThreshold: volume,
		SleepWindow:            time.Duration(sleep) * time.Millisecond,
		ErrorPercentThreshold:  errorPercent,
		RollingWindows:         rollingWindows,
	}
	circuitSettings.Store(name, setting)
}

func getSettings(name string) *Settings {
	if v, ok := circuitSettings.Load(name); ok {
		result, _ := v.(*Settings)
		return result
	}

	ConfigureCommand(name, CommandConfig{})
	return getSettings(name)
}

// GetCircuitSettings get all circuit settings
func GetCircuitSettings() map[string]*Settings {
	copy := make(map[string]*Settings)

	circuitSettings.Range(func (key, value interface{}) bool {
		name, isName := key.(string)
		setting, isSetting := key.(*Settings)
		if isName && isSetting {
			copy[name] = setting
		}
		return true
	})
	return copy
}