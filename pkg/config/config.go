package config

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/docker/docker/api"
	"github.com/docker/docker/utils"
)

const (
	configFileName = ".dockercfg"
)

type Config interface {
	Load(file string) error
	Save() error
}

type AuthConfig struct {
	Username      string `json:"username,omitempty"`
	Password      string `json:"password,omitempty"`
	Auth          string `json:"auth"`
	Email         string `json:"email"`
	ServerAddress string `json:"serveraddress,omitempty"`
}

type HostConfig struct {
	// implicit:
	//   "Host": string,
	//   "TlsVerify": bool,
	//   "CertPath": string
	hostConnectionSettings map[string]interface{} `json:"settings"`
}

type ConfigHierarchy struct {
	EnvVar  string
	JsonKey string
	Default interface{}
}

type ConfigStore struct {
	rootPath       string                `json:"-"`
	ConfigFilePath string                `json:"-"`
	Registries     map[string]AuthConfig `json:"registry,omitempty"`
	Host           HostConfig            `json:"host,omitempty"`
}

func NewConfigStore(rootPath string) (ConfigStore, error) {
	c := ConfigStore{}
	c.rootPath = rootPath
	c.ConfigFilePath = filepath.Join(rootPath, configFileName)
	if err := c.Load(c.ConfigFilePath); err != nil {
		return c, err
	}
	if c.Registries == nil {
		c.Registries = make(map[string]AuthConfig)
	}
	return c, nil
}

func (c *ConfigStore) Load(dockerConfigPath string) error {
	if _, err := os.Stat(c.ConfigFilePath); os.IsNotExist(err) {
		// not a problem - just an empty config
		return nil
	}
	data, err := ioutil.ReadFile(c.ConfigFilePath)
	if err != nil {
		switch err {
		case os.ErrPermission:
			log.Fatalf("Error reading %s: Insufficient permissions", c.ConfigFilePath)
		default:
			log.Fatalf("Unrecognized error reading %s: %s", c.ConfigFilePath, err)
		}
	}
	if json.Unmarshal(data, c); err != nil {
		log.Fatalf("Error unmarshalling %s: %s", c.ConfigFilePath, err)
	}
	c.Registries, err = transformLoadedRegistries(c.Registries)
	if err != nil {
		return err
	}
	return nil
}

func (c *ConfigStore) Save() error {
	// "blot out" these values (they get marshalled in other places afaict so we can't use `json:"-"`)
	// I'm not crazy about having custom save logic here but them's the breaks with backwards compat.
	marshalPreppedAuthConfigs := make(map[string]AuthConfig, len(c.Registries))
	for k, authConfig := range c.Registries {
		authCopy := authConfig
		authCopy.Auth = encodeAuth(&authCopy)
		authCopy.Username = ""
		authCopy.Password = ""
		authCopy.ServerAddress = ""
		marshalPreppedAuthConfigs[k] = authCopy
	}
	c.Registries = marshalPreppedAuthConfigs

	b, err := json.MarshalIndent(c, "", "\t")
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(c.ConfigFilePath, b, 0600)
	if err != nil {
		return err
	}
	return nil
}

// parse auth config information correctly
func transformLoadedRegistries(registries map[string]AuthConfig) (map[string]AuthConfig, error) {
	// WARNING: DECISION: OLD STYLE CONFIG FORMAT BEING DEPRECATED.
	// NOW ONLY JSON WILL BE HANDLED.
	var (
		err error
	)
	transformedRegistries := make(map[string]AuthConfig)
	for k, authConfig := range registries {
		authConfig.Username, authConfig.Password, err = decodeAuth(authConfig.Auth)
		if err != nil {
			return nil, err
		}
		authConfig.Auth = ""
		authConfig.ServerAddress = k
		transformedRegistries[k] = authConfig
	}
	return transformedRegistries, nil
}

// create a base64 encoded auth string to store in config
func encodeAuth(authConfig *AuthConfig) string {
	authStr := authConfig.Username + ":" + authConfig.Password
	msg := []byte(authStr)
	encoded := make([]byte, base64.StdEncoding.EncodedLen(len(msg)))
	base64.StdEncoding.Encode(encoded, msg)
	return string(encoded)
}

// decode the auth string
func decodeAuth(authStr string) (string, string, error) {
	decLen := base64.StdEncoding.DecodedLen(len(authStr))
	decoded := make([]byte, decLen)
	authByte := []byte(authStr)
	n, err := base64.StdEncoding.Decode(decoded, authByte)
	if err != nil {
		return "", "", err
	}
	if n > decLen {
		return "", "", fmt.Errorf("Something went wrong decoding auth config")
	}
	arr := strings.SplitN(string(decoded), ":", 2)
	if len(arr) != 2 {
		return "", "", fmt.Errorf("Invalid auth configuration file")
	}
	password := strings.Trim(arr[1], "\x00")
	return arr[0], password, nil
}

type LegacyConfigFile struct {
	Configs  map[string]AuthConfig `json:"configs,omitempty"`
	rootPath string
}

func (c *ConfigStore) MarshalLegacyConfigfile() ([]byte, error) {
	return json.Marshal(LegacyConfigFile{
		Configs:  c.Registries,
		rootPath: c.rootPath,
	})
}

func (hc *HostConfig) GetCertPath() string {
	return hc.getConfigValue(ConfigHierarchy{
		EnvVar:  "DOCKER_CERT_PATH",
		JsonKey: "CertPath",
		Default: filepath.Join(utils.HomeDir(), ".docker"),
	}).(string)
}

func (hc *HostConfig) GetTlsVerify() bool {
	configVal := hc.getConfigValue(ConfigHierarchy{
		EnvVar:  "DOCKER_TLS_VERIFY",
		JsonKey: "TlsVerify",
		Default: "false",
	})
	if boolVal, ok := configVal.(bool); ok {
		return boolVal
	}
	if stringVal, ok := configVal.(string); ok {
		val, err := strconv.ParseBool(stringVal)
		if err != nil {
			log.Fatalf("Error parsing TlsVerify / DOCKER_TLS_VERIFY value: %s", err)
		}
		return val
	}
	log.Fatal("Unrecognized type for TlsVerify value in config file")
	return false
}

func (hc *HostConfig) GetHost() string {
	return hc.getConfigValue(ConfigHierarchy{
		EnvVar:  "DOCKER_HOST",
		JsonKey: "Host",
		Default: fmt.Sprintf("unix://%s", api.DEFAULTUNIXSOCKET),
	}).(string)
}

// The hierarchy of configuration options flows like
// this, in order of most preferred to least preferred:
//
// Command Line Flags => Environment Variables => defaults.json => Hardcoded Defaults
func (hc *HostConfig) getConfigValue(hierarchy ConfigHierarchy) interface{} {
	envVal := os.Getenv(hierarchy.EnvVar)
	if envVal == "" {
		if hc.hostConnectionSettings != nil {
			if hc.hostConnectionSettings[hierarchy.JsonKey] != nil {
				return hc.hostConnectionSettings[hierarchy.JsonKey]
			}
		}
	} else {
		return envVal
	}
	return hierarchy.Default
}
