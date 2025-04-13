//go:build e2e

package integration

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

func TestSendNotificationServiceSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(SendNotificationTestSuite))
}

type SendNotificationTestSuite struct {
	suite.Suite
}

func (s *SendNotificationTestSuite) TestSendNotification() {
}
