package volumes

import (
	"context"
	"errors"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/volume"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/volumes/parser"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/path"
)

func newDebugLoggerMock() *mockDebugLogger {
	loggerMock := new(mockDebugLogger)
	loggerMock.On("Debugln", mock.Anything)

	return loggerMock
}

func TestErrVolumeAlreadyDefined(t *testing.T) {
	err := NewErrVolumeAlreadyDefined("test-path")
	assert.EqualError(t, err, `volume for container path "test-path" is already defined`)
}

func TestNewDefaultManager(t *testing.T) {
	logger := newDebugLoggerMock()

	m := NewManager(logger, nil, nil, ManagerConfig{})
	assert.IsType(t, &manager{}, m)
}

func newDefaultManager(config ManagerConfig) *manager {
	m := &manager{
		logger:         newDebugLoggerMock(),
		config:         config,
		managedVolumes: make(map[string]bool, 0),
	}

	return m
}

func addParser(manager *manager) *parser.MockParser {
	parserMock := new(parser.MockParser)
	parserMock.On("Path").Return(path.NewUnixPath())

	manager.parser = parserMock
	return parserMock
}

func TestDefaultManager_CreateUserVolumes_HostVolume(t *testing.T) {
	existingBinding := "/host:/duplicated"

	testCases := map[string]struct {
		volume          string
		parsedVolume    *parser.Volume
		basePath        string
		expectedBinding []string
		expectedError   error
	}{
		"no volumes specified": {
			volume:          "",
			expectedBinding: []string{existingBinding},
		},
		"volume with absolute path": {
			volume:          "/host:/volume",
			parsedVolume:    &parser.Volume{Source: "/host", Destination: "/volume"},
			expectedBinding: []string{existingBinding, "/host:/volume"},
		},
		"volume with absolute path and with basePath specified": {
			volume:          "/host:/volume",
			parsedVolume:    &parser.Volume{Source: "/host", Destination: "/volume"},
			basePath:        "/builds",
			expectedBinding: []string{existingBinding, "/host:/volume"},
		},
		"volume without absolute path and without basePath specified": {
			volume:          "/host:volume",
			parsedVolume:    &parser.Volume{Source: "/host", Destination: "volume"},
			expectedBinding: []string{existingBinding, "/host:volume"},
		},
		"volume without absolute path and with basePath specified": {
			volume:          "/host:volume",
			parsedVolume:    &parser.Volume{Source: "/host", Destination: "volume"},
			basePath:        "/builds/project",
			expectedBinding: []string{existingBinding, "/host:/builds/project/volume"},
		},
		"duplicated volume specification": {
			volume:          "/host/new:/duplicated",
			parsedVolume:    &parser.Volume{Source: "/host/new", Destination: "/duplicated"},
			expectedBinding: []string{existingBinding},
			expectedError:   NewErrVolumeAlreadyDefined("/duplicated"),
		},
		"volume with mode specified": {
			volume:          "/host/new:/my/path:ro",
			parsedVolume:    &parser.Volume{Source: "/host/new", Destination: "/my/path", Mode: "ro"},
			expectedBinding: []string{existingBinding, "/host/new:/my/path:ro"},
		},
		"root volume specified": {
			volume:          "/host/new:/:ro",
			parsedVolume:    &parser.Volume{Source: "/host/new", Destination: "/", Mode: "ro"},
			expectedBinding: []string{existingBinding},
			expectedError:   errDirectoryIsRootPath,
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			config := ManagerConfig{
				BasePath: testCase.basePath,
			}

			m := newDefaultManager(config)

			volumeParser := addParser(m)
			defer volumeParser.AssertExpectations(t)

			volumeParser.On("ParseVolume", existingBinding).
				Return(&parser.Volume{Source: "/host", Destination: "/duplicated"}, nil).
				Once()

			err := m.Create(context.Background(), existingBinding)
			require.NoError(t, err)

			if len(testCase.volume) > 0 {
				volumeParser.On("ParseVolume", testCase.volume).
					Return(testCase.parsedVolume, nil).
					Once()
			}

			err = m.Create(context.Background(), testCase.volume)
			assert.True(t, errors.Is(err, testCase.expectedError), "expected err %T, but got %T", testCase.expectedError, err)
			assert.Equal(t, testCase.expectedBinding, m.volumeBindings)
		})
	}
}

func TestDefaultManager_CreateUserVolumes_CacheVolume_Disabled(t *testing.T) {
	expectedBinding := []string{"/host:/duplicated"}

	testCases := map[string]struct {
		volume       string
		parsedVolume *parser.Volume
		basePath     string

		expectedError error
	}{
		"no volumes specified": {
			volume: "",
		},
		"volume with absolute path, without basePath and with disableCache": {
			volume:        "/volume",
			parsedVolume:  &parser.Volume{Destination: "/volume"},
			basePath:      "",
			expectedError: ErrCacheVolumesDisabled,
		},
		"volume with absolute path, with basePath and with disableCache": {
			volume:        "/volume",
			parsedVolume:  &parser.Volume{Destination: "/volume"},
			basePath:      "/builds/project",
			expectedError: ErrCacheVolumesDisabled,
		},
		"volume without absolute path, without basePath and with disableCache": {
			volume:        "volume",
			parsedVolume:  &parser.Volume{Destination: "volume"},
			expectedError: ErrCacheVolumesDisabled,
		},
		"volume without absolute path, with basePath and with disableCache": {
			volume:        "volume",
			parsedVolume:  &parser.Volume{Destination: "volume"},
			basePath:      "/builds/project",
			expectedError: ErrCacheVolumesDisabled,
		},
		"duplicated volume definition": {
			volume:        "/duplicated",
			parsedVolume:  &parser.Volume{Destination: "/duplicated"},
			basePath:      "",
			expectedError: ErrCacheVolumesDisabled,
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			config := ManagerConfig{
				BasePath:     testCase.basePath,
				DisableCache: true,
			}

			m := newDefaultManager(config)

			volumeParser := addParser(m)
			defer volumeParser.AssertExpectations(t)

			volumeParser.On("ParseVolume", "/host:/duplicated").
				Return(&parser.Volume{Source: "/host", Destination: "/duplicated"}, nil).
				Once()

			err := m.Create(context.Background(), "/host:/duplicated")
			require.NoError(t, err)

			if len(testCase.volume) > 0 {
				volumeParser.On("ParseVolume", testCase.volume).
					Return(testCase.parsedVolume, nil).
					Once()
			}

			err = m.Create(context.Background(), testCase.volume)
			assert.True(t, errors.Is(err, testCase.expectedError), "expected err %T, but got %T", testCase.expectedError, err)
			assert.Equal(t, expectedBinding, m.volumeBindings)
		})
	}
}

func TestDefaultManager_CreateUserVolumes_CacheVolume_HostBased(t *testing.T) {
	existingBinding := "/host:/duplicated"

	testCases := map[string]struct {
		volume     string
		basePath   string
		uniqueName string

		expectedBinding []string
		expectedError   error
	}{
		"volume with absolute path, without basePath": {
			volume:     "/volume",
			uniqueName: "uniq",
			expectedBinding: []string{
				existingBinding,
				"/cache/uniq/14331bf18c8e434c4b3f48a8c5cc79aa:/volume",
			},
		},
		"volume with absolute path, with basePath": {
			volume:     "/volume",
			basePath:   "/builds/project",
			uniqueName: "uniq",
			expectedBinding: []string{
				existingBinding,
				"/cache/uniq/14331bf18c8e434c4b3f48a8c5cc79aa:/volume",
			},
		},
		"volume without absolute path, without basePath": {
			volume:     "volume",
			uniqueName: "uniq",
			expectedBinding: []string{
				existingBinding,
				"/cache/uniq/210ab9e731c9c36c2c38db15c28a8d1c:volume",
			},
		},
		"volume without absolute path, with basePath": {
			volume:     "volume",
			basePath:   "/builds/project",
			uniqueName: "uniq",
			expectedBinding: []string{
				existingBinding,
				"/cache/uniq/f69aef9fb01e88e6213362a04877452d:/builds/project/volume",
			},
		},
		"duplicated volume definition": {
			volume:          "/duplicated",
			uniqueName:      "uniq",
			expectedBinding: []string{existingBinding},
			expectedError:   NewErrVolumeAlreadyDefined("/duplicated"),
		},
		"volume is root": {
			volume:          "/",
			expectedBinding: []string{existingBinding},
			expectedError:   errDirectoryIsRootPath,
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			config := ManagerConfig{
				BasePath:     testCase.basePath,
				DisableCache: false,
				CacheDir:     "/cache",
				UniqueName:   testCase.uniqueName,
			}

			m := newDefaultManager(config)

			volumeParser := addParser(m)
			defer volumeParser.AssertExpectations(t)

			volumeParser.On("ParseVolume", existingBinding).
				Return(&parser.Volume{Source: "/host", Destination: "/duplicated"}, nil).
				Once()

			err := m.Create(context.Background(), existingBinding)
			require.NoError(t, err)

			volumeParser.On("ParseVolume", testCase.volume).
				Return(&parser.Volume{Destination: testCase.volume}, nil).
				Once()

			err = m.Create(context.Background(), testCase.volume)
			assert.True(t, errors.Is(err, testCase.expectedError), "expected err %T, but got %T", testCase.expectedError, err)
			assert.Equal(t, testCase.expectedBinding, m.volumeBindings)
		})
	}
}

func TestDefaultManager_CreateUserVolumes_CacheVolume_VolumeBased(t *testing.T) {
	existingBinding := "/host:/duplicated"

	testCases := map[string]struct {
		volume     string
		basePath   string
		uniqueName string

		expectedVolumeName string
		expectedBindings   []string
		expectedError      error
	}{
		"volume with absolute path, without basePath and with existing volume": {
			volume:             "/volume",
			basePath:           "",
			uniqueName:         "uniq",
			expectedVolumeName: "uniq-cache-14331bf18c8e434c4b3f48a8c5cc79aa",
			expectedBindings: []string{
				existingBinding,
				"uniq-cache-14331bf18c8e434c4b3f48a8c5cc79aa:/volume",
			},
		},
		"volume without absolute path, with basePath": {
			volume:             "volume",
			basePath:           "/builds/project",
			uniqueName:         "uniq",
			expectedVolumeName: "uniq-cache-f69aef9fb01e88e6213362a04877452d",
			expectedBindings: []string{
				existingBinding,
				"uniq-cache-f69aef9fb01e88e6213362a04877452d:/builds/project/volume",
			},
		},
		"volume is root": {
			volume:        "/",
			basePath:      "",
			uniqueName:    "uniq",
			expectedError: errDirectoryIsRootPath,
		},
		"duplicated volume definition": {
			volume:        "/duplicated",
			uniqueName:    "uniq",
			expectedError: NewErrVolumeAlreadyDefined("/duplicated"),
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			config := ManagerConfig{
				BasePath:     testCase.basePath,
				UniqueName:   testCase.uniqueName,
				DisableCache: false,
			}

			m := newDefaultManager(config)
			volumeParser := addParser(m)
			mClient := new(docker.MockClient)
			m.client = mClient

			defer func() {
				mClient.AssertExpectations(t)
				volumeParser.AssertExpectations(t)
			}()

			volumeParser.On("ParseVolume", existingBinding).
				Return(&parser.Volume{Source: "/host", Destination: "/duplicated"}, nil).
				Once()
			volumeParser.On("ParseVolume", testCase.volume).
				Return(&parser.Volume{Destination: testCase.volume}, nil).
				Once()

			if testCase.expectedError == nil {
				mClient.On(
					"VolumeCreate",
					mock.Anything,
					mock.MatchedBy(func(v volume.VolumeCreateBody) bool {
						return v.Name == testCase.expectedVolumeName
					}),
				).
					Return(types.Volume{Name: testCase.expectedVolumeName}, nil).
					Once()
			}

			err := m.Create(context.Background(), existingBinding)
			require.NoError(t, err)

			err = m.Create(context.Background(), testCase.volume)
			if testCase.expectedError != nil {
				assert.True(t, errors.Is(err, testCase.expectedError), "expected err %T, but got %T", testCase.expectedError, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, testCase.expectedBindings, m.Binds())
		})
	}
}

func TestDefaultManager_CreateUserVolumes_CacheVolume_VolumeBased_WithError(t *testing.T) {
	testErr := errors.New("test-error")
	config := ManagerConfig{
		BasePath:   "/builds/project",
		UniqueName: "unique",
	}

	m := newDefaultManager(config)
	volumeParser := addParser(m)
	mClient := new(docker.MockClient)
	m.client = mClient

	defer func() {
		mClient.AssertExpectations(t)
		volumeParser.AssertExpectations(t)
	}()

	mClient.On(
		"VolumeCreate",
		mock.Anything,
		mock.MatchedBy(func(v volume.VolumeCreateBody) bool {
			return v.Name == "unique-cache-f69aef9fb01e88e6213362a04877452d"
		}),
	).
		Return(types.Volume{}, testErr).
		Once()

	volumeParser.On("ParseVolume", "volume").
		Return(&parser.Volume{Destination: "volume"}, nil).
		Once()

	err := m.Create(context.Background(), "volume")
	assert.True(t, errors.Is(err, testErr), "expected err %T, but got %T", testErr, err)
}

func TestDefaultManager_CreateUserVolumes_ParserError(t *testing.T) {
	testErr := errors.New("parser-test-error")
	m := newDefaultManager(ManagerConfig{})

	volumeParser := new(parser.MockParser)
	defer volumeParser.AssertExpectations(t)
	m.parser = volumeParser

	volumeParser.On("ParseVolume", "volume").
		Return(nil, testErr).
		Once()

	err := m.Create(context.Background(), "volume")
	assert.True(t, errors.Is(err, testErr), "expected err %T, but got %T", testErr, err)
}

func TestDefaultManager_CreateTemporary(t *testing.T) {
	volumeCreateErr := errors.New("volume-create")
	existingBinding := "/host:/duplicated"

	testCases := map[string]struct {
		volume          string
		volumeCreateErr error

		expectedVolumeName string
		expectedBindings   []string
		expectedError      error
	}{
		"volume created": {
			volume:             "volume",
			expectedVolumeName: "unique-cache-f69aef9fb01e88e6213362a04877452d",
			expectedBindings: []string{
				existingBinding,
				"unique-cache-f69aef9fb01e88e6213362a04877452d:/builds/project/volume",
			},
		},
		"volume root": {
			volume:        "/",
			expectedError: errDirectoryIsRootPath,
		},
		"volume creation error": {
			volume:             "volume",
			expectedVolumeName: "unique-cache-f69aef9fb01e88e6213362a04877452d",
			volumeCreateErr:    volumeCreateErr,
			expectedError:      volumeCreateErr,
		},
		"duplicated volume definition": {
			volume:        "/duplicated",
			expectedError: &ErrVolumeAlreadyDefined{},
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			config := ManagerConfig{
				BasePath:   "/builds/project",
				UniqueName: "unique",
			}

			m := newDefaultManager(config)
			volumeParser := addParser(m)
			mClient := new(docker.MockClient)
			m.client = mClient

			defer func() {
				mClient.AssertExpectations(t)
				volumeParser.AssertExpectations(t)
			}()

			volumeParser.On("ParseVolume", existingBinding).
				Return(&parser.Volume{Source: "/host", Destination: "/duplicated"}, nil).
				Once()

			if testCase.expectedVolumeName != "" {
				mClient.On(
					"VolumeCreate",
					mock.Anything,
					mock.MatchedBy(func(v volume.VolumeCreateBody) bool {
						return v.Name == testCase.expectedVolumeName
					}),
				).
					Return(types.Volume{Name: testCase.expectedVolumeName}, testCase.volumeCreateErr).
					Once()
			}

			err := m.Create(context.Background(), existingBinding)
			require.NoError(t, err)

			err = m.CreateTemporary(context.Background(), testCase.volume)
			if testCase.expectedError != nil {
				assert.True(t, errors.Is(err, testCase.expectedError), "expected err %T, but got %T", testCase.expectedError, err)
				return
			}

			assert.Equal(t, testCase.expectedError, err)
			assert.Equal(t, testCase.expectedBindings, m.Binds())
		})
	}
}

func TestDefaultManager_Binds(t *testing.T) {
	expectedElements := []string{"element1", "element2"}
	m := &manager{
		volumeBindings: expectedElements,
	}

	assert.Equal(t, expectedElements, m.Binds())
}
