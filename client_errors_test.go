package rest

import (
	"errors"
	"net/http"
	"testing"
)

func Test_isErrorRetryable(t *testing.T) {
	type args struct {
		cE *ClientError
	}
	tests := []struct {
		name              string
		args              args
		tearUp            func()
		tearDown          func()
		expectedRetryable bool
	}{
		{
			name: "Should work - nil",
			args: args{
				cE: nil,
			},
			expectedRetryable: false, // Don't matter for this case.
		},
		{
			name: "Should work - empty cE",
			args: args{
				cE: &ClientError{},
			},
			expectedRetryable: false,
		},
		{
			name: "Should work - non-retryable",
			args: args{
				cE: &ClientError{
					StatusCode: http.StatusForbidden,
				},
			},
			expectedRetryable: false,
		},
		{
			name: "Should work - retryable - http-based",
			args: args{
				cE: &ClientError{
					StatusCode: http.StatusRequestTimeout,
				},
			},
			expectedRetryable: true,
		},
		{
			name: "Should work - retryable - error-message-based",
			args: args{
				cE: &ClientError{
					Err: errors.New("test"),
				},
			},
			tearUp: func() {
				RetryableErrorMessages = append(RetryableErrorMessages, "test")
			},
			tearDown: func() {
				if len(RetryableErrorMessages) > 0 {
					RetryableErrorMessages = RetryableErrorMessages[:len(RetryableErrorMessages)-1]
				}
			},
			expectedRetryable: true,
		},
	}
	for _, tt := range tests {
		if tt.tearUp != nil {
			tt.tearUp()
		}

		t.Run(tt.name, func(t *testing.T) {
			isErrorRetryable(tt.args.cE)

			if tt.args.cE != nil {
				if tt.args.cE.Retryable != tt.expectedRetryable {
					t.Errorf("Retryable, got: %t expected: %t", tt.args.cE.Retryable, tt.expectedRetryable)
				}
			}
		})

		if tt.tearDown != nil {
			tt.tearDown()
		}
	}
}
