/*
Copyright The Helm Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package chartutil

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"

	"helm.sh/helm/pkg/chart"
	"helm.sh/helm/pkg/chart/loader"
)

const (
	// ChartfileName is the default Chart file name.
	ChartfileName = "Chart.yaml"
	// ValuesfileName is the default values file name.
	ValuesfileName = "values.yaml"
	// TemplatesDir is the relative directory name for templates.
	TemplatesDir = "templates"
	// ChartsDir is the relative directory name for charts dependencies.
	ChartsDir = "charts"
	// IgnorefileName is the name of the Helm ignore file.
	IgnorefileName = ".helmignore"
	// IngressFileName is the name of the example ingress file.
	IngressFileName = "ingress.yaml"
	// DeploymentName is the name of the example deployment file.
	DeploymentName = "deployment.yaml"
	// ServiceName is the name of the example service file.
	ServiceName = "service.yaml"
	// NotesName is the name of the example NOTES.txt file.
	NotesName = "NOTES.txt"
	// HelpersName is the name of the example NOTES.txt file.
	HelpersStarName = "helpers.star"
	HelpersLibName  = "helpers.lib.yml"
)

const defaultChartfile = `apiVersion: v1
name: %s
description: A Helm chart for Kubernetes

# A chart can be either an 'application' or a 'library' chart.
#
# Application charts are a collection of templates that can be packaged into versioned archives
# to be deployed.
#
# Library charts provide useful utilities or functions for the chart developer. They're included as
# a dependency of application charts to inject those utilities and functions into the rendering
# pipeline. Library charts do not define any templates and therefore cannot be deployed.
type: application

# This is the chart version. This version number should be incremented each time you make changes
# to the chart and its templates, including the app version.
version: 0.1.0

# This is the version number of the application being deployed. This version number should be
# incremented each time you make changes to the application.
appVersion: 1.16.0
`

const defaultValues = `#! Default values for %s.
#@data/values
---
Release:
  Name: release-name
  Service: release-sv
  Namespace: release-ns
Chart:
  Name: chart-name
  AppVersion: chart-appver
  Version: chart-ver

nameOverride:
replicaCount: 1
image:
  repository: nginx
  tag: stable
  pullPolicy: IfNotPresent
service:
  name: nginx
  type: ClusterIP
  externalPort: 80
  internalPort: 80
ingress:
  enabled: false
  hosts:
  - chart-example.local
  annotations:
    #! kubernetes.io/ingress.class: nginx
    #! kubernetes.io/tls-acme: "true"
  tls:
    #! Secrets must be manually created in the namespace.
    #! - secretName: chart-example-tls
    #!   hosts:
    #!     - chart-example.local
`

const defaultIgnore = `# Patterns to ignore when building packages.
# This supports shell glob matching, relative path matching, and
# negation (prefixed with !). Only one pattern per line.
.DS_Store
# Common VCS dirs
.git/
.gitignore
.bzr/
.bzrignore
.hg/
.hgignore
.svn/
# Common backup files
*.swp
*.bak
*.tmp
*~
# Various IDEs
.project
.idea/
*.tmproj
`

const defaultIngress = `#@ load("@ytt:data", "data")
#@ load("helpers.star", "fullname")
#@ load("helpers.lib.yml", "labels")

#@ if/end data.values.ingress.enabled:
---
apiVersion: extensions/v1beta1
kind: Ingress
metadata:
  name: #@ fullname(data.values)
  labels: #@ labels(data.values)
  #@ if/end data.values.ingress.annotations:
  annotations: #@ data.values.ingress.annotations
spec:
  rules:
    #@ for/end host in data.values.ingress.hosts:
    - host: #@ host
      http:
        paths:
          - path: /
            backend:
              serviceName: #@ fullname(data.values)
              servicePort: #@ data.values.service.externalPort
  #@ if/end data.values.ingress.tls:
  tls: #@ data.values.ingress.tls

`

const defaultDeployment = `#! example is based on the following github repo: https://bit.ly/2EoYwuN

#@ load("@ytt:data", "data")
#@ load("helpers.star", "fullname", "name")
#@ load("helpers.lib.yml", "labels")

apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: #@ fullname(data.values)
  labels: #@ labels(data.values)
spec:
  replicas: #@ data.values.replicaCount
  template:
    metadata:
      labels:
        app: #@ name(data.values)
        release: #@ data.values.Release.Name
    spec:
      containers:
      - name: #@ data.values.Chart.Name
        image: #@ "{}-{}".format(data.values.image.repository, data.values.image.tag)
        imagePullPolicy: #@ data.values.image.pullPolicy
        ports:
        - containerPort: #@ data.values.service.internalPort
        livenessProbe:
          httpGet:
            path: /
            port: #@ data.values.service.internalPort
        readinessProbe:
          httpGet:
            path: /
            port: #@ data.values.service.internalPort

`

const defaultService = `#@ load("@ytt:data", "data")
#@ load("helpers.star", "fullname", "name")
#@ load("helpers.lib.yml", "labels")

apiVersion: v1
kind: Service
metadata:
  name: #@ fullname(data.values)
  labels: #@ labels(data.values)
spec:
  type: #@ data.values.service.type
  ports:
    - port: #@ data.values.service.externalPort
      targetPort: #@ data.values.service.internalPort
      protocol: TCP
      name: #@ data.values.service.name
  selector:
    app: #@ name(data.values)
    release: #@ data.values.Release.Name

`

const defaultNotes = `(@ load("@ytt:data", "data") @)
(@ load("helpers.star", "fullname", "name") -@)

1. Get the application URL by running these commands:
(@- if data.values.ingress.enabled: @)
  (@- for h in data.values.ingress.hosts: @)
    http(@= "s" if data.values.ingress.tls else "" @)://(@= h @)
  (@ end @)
(@ elif data.values.service.type == "NodePort": @)
  export NODE_PORT=$(kubectl get --namespace (@= data.values.Release.Namespace @) -o jsonpath="{.spec.ports[0].nodePort}" services (@= fullname(data.values) @))
  export NODE_IP=$(kubectl get nodes --namespace (@= data.values.Release.Namespace @) -o jsonpath="{.items[0].status.addresses[0].address}")
  echo http://$NODE_IP:$NODE_PORT
(@- elif data.values.service.type == "LoadBalancer": @)
NOTE: It may take a few minutes for the LoadBalancer IP to be available.
You can watch the status of by running 'kubectl get svc -w (@= fullname(data.values) @)'
  export SERVICE_IP=$(kubectl get svc --namespace (@= data.values.Release.Namespace @) (@= fullname(data.values) @) -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
  echo http://$SERVICE_IP:(@= str(data.values.service.externalPort) @)
(@- elif data.values.service.type == "ClusterIP": @)
  export POD_NAME=$(kubectl get pods --namespace (@= data.values.Release.Namespace @) -l "app.kubernetes.io/name=(@= name(data.values) @),app.kubernetes.io/instance=(@= data.values.Release.Name @)" -o jsonpath="{.items[0].metadata.name}")
  echo "Visit http://127.0.0.1:8080 to use your application"
  kubectl port-forward $POD_NAME 8080:80
(@ end -@)

`

const defaultHelpersStar = `def name(vars):
  return kube_clean_name(vars.Chart.Name or vars.Values.nameOverride)
end

def fullname(vars):
  name = vars.Chart.Name or vars.Values.nameOverride
  return kube_clean_name("{}-{}".format(vars.Release.Name, name))
end

def kube_clean_name(name):
  return name[:63].rstrip("-")
end
`

const defaultHelpersLib = `#@ load("helpers.star", "name")

#@ def labels(vars):
app: #@ name(vars)
chart: #@ "{}-{}".format(vars.Chart.Name, vars.Chart.Version).replace("+", "_")
release: #@ vars.Release.Name
heritage: #@ vars.Release.Service
#@ end
`

// CreateFrom creates a new chart, but scaffolds it from the src chart.
func CreateFrom(chartfile *chart.Metadata, dest, src string) error {
	schart, err := loader.Load(src)
	if err != nil {
		return errors.Wrapf(err, "could not load %s", src)
	}

	schart.Metadata = chartfile

	var updatedTemplates []*chart.File

	for _, template := range schart.Templates {
		newData := transform(string(template.Data), schart.Name())
		updatedTemplates = append(updatedTemplates, &chart.File{Name: template.Name, Data: newData})
	}

	schart.Templates = updatedTemplates
	return SaveDir(schart, dest)
}

// Create creates a new chart in a directory.
//
// Inside of dir, this will create a directory based on the name of
// chartfile.Name. It will then write the Chart.yaml into this directory and
// create the (empty) appropriate directories.
//
// The returned string will point to the newly created directory. It will be
// an absolute path, even if the provided base directory was relative.
//
// If dir does not exist, this will return an error.
// If Chart.yaml or any directories cannot be created, this will return an
// error. In such a case, this will attempt to clean up by removing the
// new chart directory.
func Create(name, dir string) (string, error) {
	path, err := filepath.Abs(dir)
	if err != nil {
		return path, err
	}

	if fi, err := os.Stat(path); err != nil {
		return path, err
	} else if !fi.IsDir() {
		return path, errors.Errorf("no such directory %s", path)
	}

	cdir := filepath.Join(path, name)
	if fi, err := os.Stat(cdir); err == nil && !fi.IsDir() {
		return cdir, errors.Errorf("file %s already exists and is not a directory", cdir)
	}
	if err := os.MkdirAll(cdir, 0755); err != nil {
		return cdir, err
	}

	for _, d := range []string{TemplatesDir, ChartsDir} {
		if err := os.MkdirAll(filepath.Join(cdir, d), 0755); err != nil {
			return cdir, err
		}
	}

	files := []struct {
		path    string
		content []byte
	}{
		{
			// Chart.yaml
			path:    filepath.Join(cdir, ChartfileName),
			content: []byte(fmt.Sprintf(defaultChartfile, name)),
		},
		{
			// values.yaml
			path:    filepath.Join(cdir, TemplatesDir, ValuesfileName),
			content: []byte(fmt.Sprintf(defaultValues, name)),
		},
		{
			// .helmignore
			path:    filepath.Join(cdir, IgnorefileName),
			content: []byte(defaultIgnore),
		},
		{
			// ingress.yaml
			path:    filepath.Join(cdir, TemplatesDir, IngressFileName),
			content: transform(defaultIngress, name),
		},
		{
			// deployment.yaml
			path:    filepath.Join(cdir, TemplatesDir, DeploymentName),
			content: transform(defaultDeployment, name),
		},
		{
			// service.yaml
			path:    filepath.Join(cdir, TemplatesDir, ServiceName),
			content: transform(defaultService, name),
		},
		{
			// NOTES.txt
			path:    filepath.Join(cdir, TemplatesDir, NotesName),
			content: transform(defaultNotes, name),
		},
		{
			// _helpers.tpl
			path:    filepath.Join(cdir, TemplatesDir, HelpersStarName),
			content: transform(defaultHelpersStar, name),
		},
		{
			// _helpers.tpl
			path:    filepath.Join(cdir, TemplatesDir, HelpersLibName),
			content: transform(defaultHelpersLib, name),
		},
	}

	for _, file := range files {
		if _, err := os.Stat(file.path); err == nil {
			// File exists and is okay. Skip it.
			continue
		}
		if err := ioutil.WriteFile(file.path, file.content, 0644); err != nil {
			return cdir, err
		}
	}
	return cdir, nil
}

// transform performs a string replacement of the specified source for
// a given key with the replacement string
func transform(src, replacement string) []byte {
	return []byte(strings.ReplaceAll(src, "<CHARTNAME>", replacement))
}
