package resource

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const DefaultTag = "latest"

type Source struct {
	Repository string `json:"repository"`
	RawTag     Tag    `json:"tag,omitempty"`

	Username     string        `json:"username,omitempty"`
	Password     string        `json:"password,omitempty"`
	ContentTrust *ContentTrust `json:"content_trust,omitempty"`

	Debug bool `json:"debug,omitempty"`
}

type ContentTrust struct {
	Server               string `json:"server"`
	RepositoryKeyID      string `json:"repository_key_id"`
	RepositoryKey        string `json:"repository_key"`
	RepositoryPassphrase string `json:"repository_passphrase"`
	TLSKey               string `json:"tls_key"`
	TLSCert              string `json:"tls_cert"`
}

/* Create notary config directory with following structure
├── gcr-config.json
└── trust
	└── private
		└── <private-key-id>.key
└── tls
	└── <notary-host>
		├── client.cert
		└── client.key
*/
func (ct *ContentTrust) PrepareConfigDir(src string) (string, error) {
	configDir := filepath.Join(src, ".notary")
	err := os.Mkdir(configDir, os.ModePerm)
	if err != nil {
		return "", err
	}

	configObj := make(map[string]string)
	configObj["server_url"] = ct.Server
	configObj["root_passphrase"] = ""
	configObj["repository_passphrase"] = ct.RepositoryPassphrase
	configData, err := json.Marshal(configObj)
	if err != nil {
		return "", err
	}
	err = ioutil.WriteFile(filepath.Join(configDir, "gcr-config.json"), configData, 0644)
	if err != nil {
		return "", err
	}

	u, err := url.Parse(ct.Server)
	if err != nil {
		return "", err
	}
	privateDir := filepath.Join(configDir, "trust", "private")
	err = os.MkdirAll(privateDir, os.ModePerm)
	if err != nil {
		return "", err
	}
	repoKey := fmt.Sprintf("%s.key", ct.RepositoryKeyID)
	err = ioutil.WriteFile(filepath.Join(privateDir, repoKey), []byte(ct.RepositoryKey), 0600)
	if err != nil {
		return "", err
	}

	if u.Host != "" {
		certDir := filepath.Join(configDir, "tls", u.Host)
		err = os.MkdirAll(certDir, os.ModePerm)
		if err != nil {
			return "", err
		}
		err = ioutil.WriteFile(filepath.Join(certDir, "client.cert"), []byte(ct.TLSCert), 0644)
		if err != nil {
			return "", err
		}
		err = ioutil.WriteFile(filepath.Join(certDir, "client.key"), []byte(ct.TLSKey), 0644)
		if err != nil {
			return "", err
		}
	}
	return configDir, nil
}

func (source *Source) Name() string {
	return fmt.Sprintf("%s:%s", source.Repository, source.Tag())
}

func (source *Source) Tag() string {
	if source.RawTag != "" {
		return string(source.RawTag)
	}

	return DefaultTag
}

func (source *Source) Metadata() []MetadataField {
	return []MetadataField{
		MetadataField{
			Name:  "repository",
			Value: source.Repository,
		},
		MetadataField{
			Name:  "tag",
			Value: source.Tag(),
		},
	}
}

func (source *Source) MetadataWithAdditionalTags(tags []string) []MetadataField {
	return []MetadataField{
		MetadataField{
			Name:  "repository",
			Value: source.Repository,
		},
		MetadataField{
			Name:  "tags",
			Value: strings.Join(append(tags, source.Tag()), " "),
		},
	}
}

// Tag refers to a tag for an image in the registry.
type Tag string

// UnmarshalJSON accepts numeric and string values.
func (tag *Tag) UnmarshalJSON(b []byte) error {
	var n json.Number
	err := json.Unmarshal(b, &n)
	if err != nil {
		return err
	}

	*tag = Tag(n.String())

	return nil
}

type Version struct {
	Digest string `json:"digest"`
}

type MetadataField struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type GetParams struct {
	RawFormat string `json:"format"`
}

func (p GetParams) Format() string {
	if p.RawFormat == "" {
		return "rootfs"
	}

	return p.RawFormat
}

type PutParams struct {
	Image          string `json:"image"`
	AdditionalTags string `json:"additional_tags"`
}

func (p *PutParams) ParseTags(src string) ([]string, error) {
	if p.AdditionalTags == "" {
		return nil, nil
	}

	filepath := filepath.Join(src, p.AdditionalTags)

	content, err := ioutil.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file at %q: %s", filepath, err)
	}

	return strings.Fields(string(content)), nil
}
