package fxTrigger

import (
	"context"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"os"
	"testing"
)

var testYaml = `
toEmail: test@gmail.com
`
var testYAMLFile, _ = createTempFile("config.yaml", []byte(testYaml))

func TestHandler(t *testing.T) {
	tests := []struct {
		name       string
		configPath string
	}{
		{name: "success",
			configPath: testYAMLFile.Name()},
	}

	for _, test := range tests {
		tt := test
		t.Run(tt.name, func(t *testing.T) {
			require.NoError(t, os.Setenv(configPathKey, tt.configPath))
			err := Handler(context.Background(), CustomEvent{})

			require.NoError(t, err)
		})
	}

}

func createTempFile(fileName string, content []byte) (f *os.File, err error) {
	file, _ := ioutil.TempFile(os.TempDir(), fileName)
	defer func(file *os.File) {
		_ = file.Close()
	}(file)

	_, err = file.Write(content)
	if err != nil {
		return nil, err
	}

	return file, nil
}
