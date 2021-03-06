package environment_test

import (
	"context"
	"github.com/fabric8-services/fabric8-tenant/environment"
	"github.com/fabric8-services/fabric8-tenant/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/h2non/gock.v1"
	"regexp"
	"strings"
	"testing"
)

var defaultLocationTempl = `apiVersion: v1
kind: Template
metadata:
  name: fabric8-tenant-${DEPLOY_TYPE}
objects:
- apiVersion: v1
  kind: ProjectRequest
  metadata:
    labels:
      test: default-location
      version: ${COMMIT}
    name: ${USER_NAME}-${DEPLOY_TYPE}`

var customLocationTempl = `apiVersion: v1
kind: Template
metadata:
  name: fabric8-tenant-${DEPLOY_TYPE}
objects:
- apiVersion: v1
  kind: ProjectRequest
  metadata:
    labels:
      test: custom-location
      version: ${COMMIT}
    name: ${USER_NAME}-${DEPLOY_TYPE}`

func TestGetAllTemplatesForAllTypes(t *testing.T) {
	// given
	service := environment.NewService("", "", "")
	setTemplateVersions()
	vars := map[string]string{
		"USER_NAME": "dev",
	}

	for _, envType := range environment.DefaultEnvTypes {
		// when
		env, err := service.GetEnvData(context.Background(), envType)
		require.NoError(t, err)
		objects, err := env.Templates[0].Process(vars)

		// then
		require.NoError(t, err)
		assert.Equal(t, env.Name, envType)
		if envType == "che" || envType == "jenkins" {
			assert.Len(t, env.Templates, 2)
			assert.Contains(t, env.Templates[0].Filename, envType)
			assert.Contains(t, env.Templates[1].Filename, "quotas")
			if envType == "jenkins" {
				assert.Equal(t, "567efg", environment.GetLabelVersion(objects[0]))
			} else {
				if strings.Contains(env.Templates[0].Filename, "mt") {
					assert.Equal(t, "234bcd", environment.GetLabelVersion(objects[0]))
				} else {
					assert.Equal(t, "123abc", environment.GetLabelVersion(objects[0]))
				}
			}
		} else if envType == "user" {
			assert.Len(t, env.Templates, 1)
			assert.Contains(t, env.Templates[0].Filename, envType)
			assert.Equal(t, "345cde", environment.GetLabelVersion(objects[0]))
		} else {
			assert.Len(t, env.Templates, 1)
			assert.Contains(t, env.Templates[0].Filename, "deploy")
			assert.Equal(t, "456def", environment.GetLabelVersion(objects[0]))
		}

		for _, template := range env.Templates {
			assert.NotEmpty(t, template.Content)
		}
	}
}

func TestAllTemplatesHaveNecessaryData(t *testing.T) {
	// given
	setTemplateVersions()
	service := environment.NewService("", "", "")
	vars := map[string]string{
		"USER_NAME": "dev",
	}

	for _, envType := range environment.DefaultEnvTypes {
		nsName := "dev-" + envType
		if envType == string(tenant.TypeUser) {
			nsName = "dev"
		}

		// when
		env, err := service.GetEnvData(context.Background(), envType)
		require.NoError(t, err)
		objects, err := env.Templates[0].Process(vars)
		require.NoError(t, err)

		//then
		for _, obj := range objects {
			assert.Regexp(t, regexp.MustCompile(`[1-7]{3}[a-g]{3}`), environment.GetLabelVersion(obj))
			if environment.GetKind(obj) != environment.ValKindProjectRequest {
				assert.Contains(t, environment.GetNamespace(obj), nsName)
			} else {
				assert.Contains(t, environment.GetName(obj), nsName)
			}
		}
	}
}

func setTemplateVersions() {
	environment.VersionFabric8TenantCheFile = "123abc"
	environment.VersionFabric8TenantCheMtFile = "234bcd"
	environment.VersionFabric8TenantUserFile = "345cde"
	environment.VersionFabric8TenantDeployFile = "456def"
	environment.VersionFabric8TenantJenkinsFile = "567efg"
}

func TestDownloadFromGivenBlob(t *testing.T) {
	// given
	defer gock.OffAll()
	gock.New("https://github.com").
		Get("fabric8-services/fabric8-tenant/blob/987654321/environment/templates/fabric8-tenant-deploy.yml").
		Reply(200).
		BodyString(defaultLocationTempl)
	setTemplateVersions()
	service := environment.NewService("", "987654321", "")

	// when
	envData, err := service.GetEnvData(context.Background(), "run")

	// then
	require.NoError(t, err)
	vars := map[string]string{
		"USER_NAME": "dev",
	}
	objects, err := envData.Templates[0].Process(vars)
	require.NoError(t, err)
	assert.Len(t, objects, 1)
	assert.Equal(t, environment.GetLabel(objects[0], "test"), "default-location")
	assert.Equal(t, environment.GetLabelVersion(objects[0]), "987654321")
}

func TestDownloadFromGivenBlobLocatedInCustomLocation(t *testing.T) {
	// given
	defer gock.OffAll()
	gock.New("http://my.git.com").
		Get("my-services/my-tenant/blob/987cba/any/path/fabric8-tenant-deploy.yml").
		Reply(200).
		BodyString(customLocationTempl)
	setTemplateVersions()
	service := environment.NewService("http://my.git.com/my-services/my-tenant", "987cba", "any/path")

	// when
	envData, err := service.GetEnvData(context.Background(), "run")

	// then
	require.NoError(t, err)
	vars := map[string]string{
		"USER_NAME": "dev",
	}
	objects, err := envData.Templates[0].Process(vars)
	require.NoError(t, err)
	assert.Len(t, objects, 1)
	assert.Equal(t, environment.GetLabel(objects[0], "test"), "custom-location")
	assert.Equal(t, environment.GetLabelVersion(objects[0]), "987cba")
}

var dnsRegExp = "^[a-z0-9]([-a-z0-9]*[a-z0-9])?$"

func TestCreateUsername(t *testing.T) {
	assertName(t, "some", "some@email.com")
	assertName(t, "so-me", "so-me@email.com")
	assertName(t, "some", "some")
	assertName(t, "so-me", "so-me")
	assertName(t, "so-me", "so_me")
	assertName(t, "so-me", "so me")
	assertName(t, "so-me", "so me@email.com")
	assertName(t, "so-me", "so.me")
	assertName(t, "so-me", "so?me")
	assertName(t, "so-me", "so:me")
	assertName(t, "some1", "some1")
	assertName(t, "so1me1", "so1me1")
}

func assertName(t *testing.T, expected, username string) {
	assert.Regexp(t, dnsRegExp, environment.RetrieveUserName(username))
	assert.Equal(t, expected, environment.RetrieveUserName(username))
}
