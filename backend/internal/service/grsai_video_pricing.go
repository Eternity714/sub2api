package service

import "fmt"

type GrsaiTaskPrice struct {
	Mode       BillingMode
	UnitPrice  float64
	Resolution string
}

func ValidateGrsaiVideoRequest(model, resolution string, duration int) error {
	if duration <= 0 {
		return fmt.Errorf("%w: duration must be a positive integer", ErrGrsaiInvalidRequest)
	}
	if model == "minimax-h3" {
		if resolution != "480p" && resolution != "768p" && resolution != "1080p" {
			return fmt.Errorf("%w: unsupported resolution", ErrGrsaiInvalidRequest)
		}
		if duration > 15 || (resolution == "1080p" && duration > 10) {
			return fmt.Errorf("%w: unsupported duration", ErrGrsaiInvalidRequest)
		}
	}
	return nil
}
