package registry

import (
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func TestIdentify_Quay(t *testing.T) {
	os.Clearenv()
	_ = os.Setenv("QUAY_REPOSITORY", "repo")
	_ = os.Setenv("QUAY_USERNAME", "user")
	_ = os.Setenv("QUAY_PASSWORD", "pass")

	docker := &MockDocker{}
	result := Identify()
	assert.NotNil(t, result)
	assert.Equal(t, "quay.io/repo", result.RegistryUrl())
	result.Login(docker)
	assert.Equal(t, "user", docker.Username)
	assert.Equal(t, "pass", docker.Password)
	assert.Equal(t, "quay.io", docker.ServerAddress)
}
