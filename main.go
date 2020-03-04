package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"runtime"
	"strings"

	"github.com/alecthomas/kingpin"
	foundation "github.com/estafette/estafette-foundation"
	"github.com/rs/zerolog/log"
)

var (
	appgroup  string
	app       string
	version   string
	branch    string
	revision  string
	buildDate string
	goVersion = runtime.Version()
)

var (
	// flags
	paramsJSON      = kingpin.Flag("params", "Extension parameters, created from custom properties.").Envar("ESTAFETTE_EXTENSION_CUSTOM_PROPERTIES").Required().String()
	credentialsJSON = kingpin.Flag("credentials", "GKE credentials configured at service level, passed in to this trusted extension.").Envar("ESTAFETTE_CREDENTIALS_KUBERNETES_ENGINE").Required().String()

	// optional flags
	gitName       = kingpin.Flag("git-name", "Repository name, used as application name if not passed explicitly and app label not being set.").Envar("ESTAFETTE_GIT_NAME").String()
	appLabel      = kingpin.Flag("app-name", "App label, used as application name if not passed explicitly.").Envar("ESTAFETTE_LABEL_APP").String()
	buildVersion  = kingpin.Flag("build-version", "Version number, used if not passed explicitly.").Envar("ESTAFETTE_BUILD_VERSION").String()
	releaseName   = kingpin.Flag("release-name", "Name of the release section, which is used by convention to resolve the credentials.").Envar("ESTAFETTE_RELEASE_NAME").String()
	releaseAction = kingpin.Flag("release-action", "Name of the release action, to control the type of release.").Envar("ESTAFETTE_RELEASE_ACTION").String()
	releaseID     = kingpin.Flag("release-id", "ID of the release, to use as a label.").Envar("ESTAFETTE_RELEASE_ID").String()
	triggeredBy   = kingpin.Flag("triggered-by", "The user id of the person triggering the release.").Envar("ESTAFETTE_TRIGGER_MANUAL_USER_ID").String()
)

func main() {

	// parse command line parameters
	kingpin.Parse()

	// init log format from envvar ESTAFETTE_LOG_FORMAT
	foundation.InitLoggingFromEnv(appgroup, app, version, branch, revision, buildDate)

	// create context to cancel commands on sigterm
	ctx := foundation.InitCancellationContext(context.Background())

	// put all estafette labels in map
	log.Info().Msg("Getting all estafette labels from envvars...")
	estafetteLabels := map[string]string{}
	for _, e := range os.Environ() {
		kvPair := strings.SplitN(e, "=", 2)

		if len(kvPair) == 2 {
			envvarName := kvPair[0]
			envvarValue := kvPair[1]

			if strings.HasPrefix(envvarName, "ESTAFETTE_LABEL_") && !strings.HasSuffix(envvarName, "_DNS_SAFE") {
				// strip prefix and convert to lowercase
				key := strings.ToLower(strings.Replace(envvarName, "ESTAFETTE_LABEL_", "", 1))
				estafetteLabels[key] = envvarValue
			}
		}
	}

	log.Info().Msg("Unmarshalling credentials parameter...")
	var credentialsParam CredentialsParam
	err := json.Unmarshal([]byte(*paramsJSON), &credentialsParam)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed unmarshalling credential parameter")
	}

	log.Info().Msg("Setting default for credential parameter...")
	credentialsParam.SetDefaults(*releaseName)

	log.Info().Msg("Validating required credential parameter...")
	valid, errors := credentialsParam.ValidateRequiredProperties()
	if !valid {
		log.Fatal().Msgf("Not all valid fields are set: %v", errors)
	}

	log.Info().Msg("Unmarshalling injected credentials...")
	var credentials []GKECredentials
	err = json.Unmarshal([]byte(*credentialsJSON), &credentials)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed unmarshalling injected credentials")
	}

	log.Info().Msgf("Checking if credential %v exists...", credentialsParam.Credentials)
	credential := GetCredentialsByName(credentials, credentialsParam.Credentials)
	if credential == nil {
		log.Fatal().Err(err).Msgf("Credential with name %v does not exist.", credentialsParam.Credentials)
	}

	var params Params
	if credential.AdditionalProperties.Defaults != nil {
		log.Info().Msgf("Using defaults from credential %v...", credentialsParam.Credentials)
		params = *credential.AdditionalProperties.Defaults
	}

	log.Info().Msg("Unmarshalling parameters / custom properties...")
	err = json.Unmarshal([]byte(*paramsJSON), &params)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed unmarshalling parameters")
	}

	log.Info().Msg("Setting defaults for parameters that are not set in the manifest...")
	params.SetDefaults(*gitName, *appLabel, *buildVersion, *releaseName, *releaseAction, estafetteLabels)

	log.Info().Msg("Validating required parameters...")
	valid, errors, warnings := params.ValidateRequiredProperties()
	if !valid {
		log.Fatal().Err(err).Msg("Not all valid fields are set")
	}

	for _, warning := range warnings {
		log.Printf("Warning: %s", warning)
	}

	log.Info().Msg("Retrieving service account email from credentials...")
	var keyFileMap map[string]interface{}
	err = json.Unmarshal([]byte(credential.AdditionalProperties.ServiceAccountKeyfile), &keyFileMap)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed unmarshalling service account keyfile")
	}
	var saClientEmail string
	if saClientEmailIntfc, ok := keyFileMap["client_email"]; !ok {
		log.Fatal().Err(err).Msg("Field client_email missing from service account keyfile")
	} else {
		if t, aok := saClientEmailIntfc.(string); !aok {
			log.Fatal().Err(err).Msg("Field client_email not of type string")
		} else {
			saClientEmail = t
		}
	}

	log.Info().Msgf("Storing gke credential %v on disk...", credentialsParam.Credentials)
	err = ioutil.WriteFile("/key-file.json", []byte(credential.AdditionalProperties.ServiceAccountKeyfile), 0600)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed writing service account keyfile")
	}

	log.Info().Msg("Authenticating to google cloud")
	foundation.RunCommandWithArgs(ctx, "gcloud", []string{"auth", "activate-service-account", saClientEmail, "--key-file", "/key-file.json"})

	log.Info().Msg("Setting gcloud account")
	foundation.RunCommandWithArgs(ctx, "gcloud", []string{"config", "set", "account", saClientEmail})

	log.Info().Msg("Setting gcloud project")
	foundation.RunCommandWithArgs(ctx, "gcloud", []string{"config", "set", "project", credential.AdditionalProperties.Project})

	log.Info().Msg("Setting gcloud project")
	foundation.RunCommandWithArgs(ctx, "gcloud", []string{"config", "set", "project", credential.AdditionalProperties.Project})

	// prepare to pass labels as argument
	estafetteLabels = sanitizeLabels(estafetteLabels)
	labelParams := []string{}
	for k, v := range estafetteLabels {
		labelParams = append(labelParams, fmt.Sprintf("%v=%v", k, v))
	}

	arguments := []string{
		"functions",
		"deploy", params.App,
		"--region", credential.AdditionalProperties.Region,
		"--memory", params.Memory,
		"--source", params.Source,
		"--timeout", fmt.Sprintf("%vs", params.TimeoutSeconds),
		"--runtime", params.Runtime,
		"--update-labels", strings.Join(labelParams, ","),
		"--ingress-settings", params.IngressSettings}

	if len(params.EnvironmentVariables) > 0 {

		// prepare to pass environment variables as argument
		envvarParams := []string{}
		for k, v := range params.EnvironmentVariables {
			envvarParams = append(envvarParams, fmt.Sprintf("%v=%v", k, v))
		}

		arguments = append(arguments, "--set-env-vars", strings.Join(envvarParams, ","))
	}

	if params.ServiceAccount != "" {
		arguments = append(arguments, "--service-account", params.ServiceAccount)
	}

	if params.Trigger == "bucket" {
	    arguments = append(arguments, "--trigger-bucket", params.TriggerValue)
	} else {
	    arguments = append(arguments, "--trigger-http")
	}

	if params.DryRun {

		log.Info().Msgf("Dry run cloud function %v deployment...", params.App)
		log.Info().Msgf("gcloud %v", arguments)

	} else {

		log.Info().Msgf("Deploying cloud function %v...", params.App)
		foundation.RunCommandWithArgs(ctx, "gcloud", arguments)

		// gcloud functions deploy (NAME : --region=REGION)
		// [--entry-point=ENTRY_POINT] [--memory=MEMORY] [--retry]
		// [--runtime=RUNTIME] [--service-account=SERVICE_ACCOUNT]
		// [--source=SOURCE] [--stage-bucket=STAGE_BUCKET] [--timeout=TIMEOUT]
		// [--update-labels=[KEY=VALUE,...]]
		// [--clear-env-vars | --env-vars-file=FILE_PATH
		//   | --set-env-vars=[KEY=VALUE,...]
		//   | --remove-env-vars=[KEY,...] --update-env-vars=[KEY=VALUE,...]]
		// [--clear-labels | --remove-labels=[KEY,...]]
		// [--trigger-bucket=TRIGGER_BUCKET | --trigger-http
		//   | --trigger-topic=TRIGGER_TOPIC
		//   | --trigger-event=EVENT_TYPE --trigger-resource=RESOURCE]
		// [GCLOUD_WIDE_FLAG ...]

		// NAME
		//     gcloud functions deploy - create or update a Google Cloud Function

		// SYNOPSIS
		//     gcloud functions deploy (NAME : --region=REGION)
		//         [--entry-point=ENTRY_POINT] [--memory=MEMORY] [--retry]
		//         [--runtime=RUNTIME] [--service-account=SERVICE_ACCOUNT]
		//         [--source=SOURCE] [--stage-bucket=STAGE_BUCKET] [--timeout=TIMEOUT]
		//         [--update-labels=[KEY=VALUE,...]]
		//         [--clear-env-vars | --env-vars-file=FILE_PATH
		//           | --set-env-vars=[KEY=VALUE,...]
		//           | --remove-env-vars=[KEY,...] --update-env-vars=[KEY=VALUE,...]]
		//         [--clear-labels | --remove-labels=[KEY,...]]
		//         [--trigger-bucket=TRIGGER_BUCKET | --trigger-http
		//           | --trigger-topic=TRIGGER_TOPIC
		//           | --trigger-event=EVENT_TYPE --trigger-resource=RESOURCE]
		//         [GCLOUD_WIDE_FLAG ...]

		// DESCRIPTION
		//     Create or update a Google Cloud Function.

		// POSITIONAL ARGUMENTS
		// 		Function resource - The Cloud function name to deploy. The arguments in
		// 		this group can be used to specify the attributes of this resource. (NOTE)
		// 		Some attributes are not given arguments in this group but can be set in
		// 		other ways. To set the [project] attribute: provide the argument [NAME] on
		// 		the command line with a fully specified name; provide the argument
		// 		[--project] on the command line; set the property [core/project]. This
		// 		must be specified.

		// 			NAME
		// 				 ID of the function or fully qualified identifier for the function.
		// 				 This positional must be specified if any of the other arguments in
		// 				 this group are specified.

		// 			--region=REGION
		// 				 The Cloud region for the function. Overrides the default
		// 				 functions/region property value for this command invocation.

		// FLAGS
		// 		--entry-point=ENTRY_POINT
		// 			 Name of a Google Cloud Function (as defined in source code) that will
		// 			 be executed. Defaults to the resource name suffix, if not specified.
		// 			 For backward compatibility, if function with given name is not found,
		// 			 then the system will try to use function named "function". For Node.js
		// 			 this is name of a function exported by the module specified in
		// 			 source_location.

		// 		--memory=MEMORY
		// 			 Limit on the amount of memory the function can use.

		// 			 Allowed values are: 128MB, 256MB, 512MB, 1024MB, and 2048MB. By
		// 			 default, a new function is limited to 256MB of memory. When deploying
		// 			 an update to an existing function, the function will keep its old
		// 			 memory limit unless you specify this flag.

		// 		--retry
		// 			 If specified, then the function will be retried in case of a failure.

		// 		--runtime=RUNTIME
		// 			 Runtime in which to run the function.

		// 			 Required when deploying a new function; optional when updating an
		// 			 existing function.

		// 			 Choices:

		// 			 ◆ nodejs8: Node.js 8
		// 			 ◆ nodejs10: Node.js 10
		// 			 ◆ python37: Python 3.7
		// 			 ◆ go111: Go 1.11
		// 			 ◆ nodejs6: Node.js 6 (deprecated)

		// 		--service-account=SERVICE_ACCOUNT
		// 			 The email address of the IAM service account associated with the
		// 			 function at runtime. The service account represents the identity of the
		// 			 running function, and determines what permissions the function has.

		// 			 If not provided, the function will use the project's default service
		// 			 account.

		// 		--source=SOURCE
		// 			 Location of source code to deploy.

		// 			 Location of the source can be one of the following three options:

		// 			 ◆ Source code in Google Cloud Storage (must be a .zip archive),
		// 			 ◆ Reference to source repository or,
		// 			 ◆ Local filesystem path (root directory of function source).

		// 	 Note that if you do not specify the --source flag:

		// 		 ▪ Current directory will be used for new function deployments.
		// 		 ▪ If the function is previously deployed using a local filesystem path,
		// 	 then function's source code will be updated using the current directory.
		// 		 ▪ If the function is previously deployed using a Google Cloud Storage
		// 	 location or a source repository, then the function's source code will not
		// 	 be updated.

		// 	 The value of the flag will be interpreted as a Cloud Storage location, if
		// 	 it starts with gs://.

		// 	 The value will be interpreted as a reference to a source repository, if it
		// 	 starts with https://.

		// 	 Otherwise, it will be interpreted as the local filesystem path. When
		// 	 deploying source from the local filesystem, this command skips files
		// 	 specified in the .gcloudignore file (see gcloud topic gcloudignore for more
		// 	 information). If the .gcloudignore file doesn't exist, the command will try
		// 	 to create it.

		// 	 The minimal source repository URL is:
		// 	 https://source.developers.google.com/projects/${PROJECT}/repos/${REPO}

		// 	 By using the URL above, sources from the root directory of the repository
		// 	 on the revision tagged master will be used.

		// 	 If you want to deploy from a revision different from master, append one of
		// 	 the following three sources to the URL:

		// 		 ▪ /revisions/${REVISION},
		// 		 ▪ /moveable-aliases/${MOVEABLE_ALIAS},
		// 		 ▪ /fixed-aliases/${FIXED_ALIAS}.

		// 	 If you'd like to deploy sources from a directory different from the root,
		// 	 you must specify a revision, a moveable alias, or a fixed alias, as above,
		// 	 and append /paths/${PATH_TO_SOURCES_DIRECTORY} to the URL.

		// 	 Overall, the URL should match the following regular expression:

		// 			 ^https://source\.developers\.google\.com/projects/
		// 			 (?<accountId>[^/]+)/repos/(?<repoName>[^/]+)
		// 			 (((/revisions/(?<commit>[^/]+))|(/moveable-aliases/(?<branch>[^/]+))|
		// 			 (/fixed-aliases/(?<tag>[^/]+)))(/paths/(?<path>.*))?)?$

		// 	 An example of a validly formatted source repository URL is:

		// 			 https://source.developers.google.com/projects/123456789/repos/testrepo/
		// 			 moveable-aliases/alternate-branch/paths/path-to=source

		// 		--stage-bucket=STAGE_BUCKET
		// 			 When deploying a function from a local directory, this flag's value is
		// 			 the name of the Google Cloud Storage bucket in which source code will
		// 			 be stored. Note that if you set the --stage-bucket flag when deploying
		// 			 a function, you will need to specify --source or --stage-bucket in
		// 			 subsequent deployments to update your source code. To use this flag
		// 			 successfully, the account in use must have permissions to write to this
		// 			 bucket. For help granting access, refer to this guide:
		// 			 https://cloud.google.com/storage/docs/access-control/

		// 		--timeout=TIMEOUT
		// 			 The function execution timeout, e.g. 30s for 30 seconds. Defaults to
		// 			 original value for existing function or 60 seconds for new functions.
		// 			 Cannot be more than 540s. See $ gcloud topic datetimes for information
		// 			 on duration formats.

		// 		--update-labels=[KEY=VALUE,...]
		// 			 List of label KEY=VALUE pairs to update. If a label exists its value is
		// 			 modified, otherwise a new label is created.

		// 			 Keys must start with a lowercase character and contain only hyphens
		// 			 (-), underscores (_), lowercase characters, and numbers. Values must
		// 			 contain only hyphens (-), underscores (_), lowercase characters, and
		// 			 numbers.

		// 			 Label keys starting with deployment are reserved for use by deployment
		// 			 tools and cannot be specified manually.

		// 		At most one of these may be specified:

		// 			--clear-env-vars
		// 				 Remove all environment variables.

		// 			--env-vars-file=FILE_PATH
		// 				 Path to a local YAML file with definitions for all environment
		// 				 variables. All existing environment variables will be removed before
		// 				 the new environment variables are added.

		// 			--set-env-vars=[KEY=VALUE,...]
		// 				 List of key-value pairs to set as environment variables. All existing
		// 				 environment variables will be removed first.

		// 			Only --update-env-vars and --remove-env-vars can be used together. If
		// 			both are specified, --remove-env-vars will be applied first.

		// 				--remove-env-vars=[KEY,...]
		// 					 List of environment variables to be removed.

		// 				--update-env-vars=[KEY=VALUE,...]
		// 					 List of key-value pairs to set as environment variables.
		// 					 At most one of these may be specified:

		// 					 --clear-labels
		// 							Remove all labels. If --update-labels is also specified then
		// 							--clear-labels is applied first.

		// 							For example, to remove all labels:

		// 									$ gcloud functions deploy --clear-labels

		// 							To set the labels to exactly "foo" and "baz":

		// 									$ gcloud functions deploy --clear-labels \
		// 										--update-labels foo=bar,baz=qux

		// 					 --remove-labels=[KEY,...]
		// 							List of label keys to remove. If a label does not exist it is
		// 							silently ignored.Label keys starting with deployment are reserved for
		// 							use by deployment tools and cannot be specified manually.

		// 				 If you don't specify a trigger when deploying an update to an existing
		// 				 function it will keep its current trigger. You must specify
		// 				 --trigger-topic, --trigger-bucket, --trigger-http or (--trigger-event AND
		// 				 --trigger-resource) when deploying a new function. At most one of these
		// 				 may be specified:

		// 					 --trigger-bucket=TRIGGER_BUCKET
		// 							Google Cloud Storage bucket name. Every change in files in this
		// 							bucket will trigger function execution.
		// 							--trigger-http
		// 							Function will be assigned an endpoint, which you can view by using
		// 							the describe command. Any HTTP request (of a supported type) to the
		// 							endpoint will trigger function execution. Supported HTTP request
		// 							types are: POST, PUT, GET, DELETE, and OPTIONS.

		// 					 --trigger-topic=TRIGGER_TOPIC
		// 							Name of Pub/Sub topic. Every message published in this topic will
		// 							trigger function execution with message contents passed as input
		// 							data.

		// 					 --trigger-event=EVENT_TYPE
		// 							Specifies which action should trigger the function. For a list of
		// 							acceptable values, call gcloud functions event-types list.

		// 					 --trigger-resource=RESOURCE
		// 							Specifies which resource from --trigger-event is being observed. E.g.
		// 							if --trigger-event is
		// 							providers/cloud.storage/eventTypes/object.change, --trigger-resource
		// 							must be a bucket name. For a list of expected resources, call gcloud
		// 							functions event-types list.

		// 		GCLOUD WIDE FLAGS
		// 				These flags are available to all commands: --account, --configuration,
		// 				--flags-file, --flatten, --format, --help, --impersonate-service-account,
		// 				--log-http, --project, --quiet, --trace-token, --user-output-enabled,
		// 				--verbosity. Run $ gcloud help for details.

		// 		NOTES
		// 				This variant is also available:

		log.Info().Msgf("Describing cloud function %v...", params.App)

		describeArguments := []string{
			"functions",
			"describe", params.App,
			"--region", credential.AdditionalProperties.Region}

		foundation.RunCommandWithArgs(ctx, "gcloud", describeArguments)
	}
}

// a valid label must be an empty string or consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyValue',  or 'my_value',  or '12345', regex used for validation is '(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?')
func sanitizeLabel(value string) string {

	// Valid label values must be 63 characters or less and must be empty or begin and end with an alphanumeric character ([a-z0-9A-Z])
	// with dashes (-), underscores (_), dots (.), and alphanumerics between.

	// replace all invalid characters with a hyphen
	reg := regexp.MustCompile(`[^a-zA-Z0-9-_.]+`)
	value = reg.ReplaceAllString(value, "-")

	// replace double hyphens with a single one
	value = strings.Replace(value, "--", "-", -1)

	// ensure it starts with an alphanumeric character
	reg = regexp.MustCompile(`^[-_.]+`)
	value = reg.ReplaceAllString(value, "")

	// maximize length at 63 characters
	if len(value) > 63 {
		value = value[:63]
	}

	// ensure it ends with an alphanumeric character
	reg = regexp.MustCompile(`[-_.]+$`)
	value = reg.ReplaceAllString(value, "")

	return value
}

func sanitizeLabels(labels map[string]string) (sanitizedLabels map[string]string) {
	sanitizedLabels = make(map[string]string, len(labels))
	for k, v := range labels {
		sanitizedLabels[k] = sanitizeLabel(v)
	}
	return
}
