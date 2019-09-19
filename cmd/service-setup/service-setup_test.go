package main

import (
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"os"
	"testing"
)

func Test(t *testing.T) {
	oldPwd, _ := os.Getwd()
	name, _ := ioutil.TempDir(os.TempDir(), "build-tools")
	defer os.RemoveAll(name)
	err := os.Chdir(name)
	assert.NoError(t, err)
	defer os.Chdir(oldPwd)

	os.Clearenv()
	_ = os.Setenv("REGISTRY", "dockerhub")

	exitFunc = func(code int) {
		assert.Equal(t, -1, code)
	}
	os.Args = []string{"service-setup"}
	main()
}
