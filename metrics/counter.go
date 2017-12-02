package metrics

import (

)

type MetricsClient interface {
	StartMeasure()
	EndMeasure()
}