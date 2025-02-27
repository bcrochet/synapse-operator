/*
Copyright 2021.

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

package mautrixsignal

import (
	"context"
	"errors"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/opdev/subreconciler"
	synapsev1alpha1 "github.com/opdev/synapse-operator/apis/synapse/v1alpha1"
	"github.com/opdev/synapse-operator/helpers/reconcile"
	"github.com/opdev/synapse-operator/helpers/utils"
)

// reconcileMautrixSignalConfigMap is a function of type FnWithRequest, to
// be called in the main reconciliation loop.
//
// It reconciles the mautrix-signal ConfigMap to its desired state. It is
// called only if the user hasn't provided its own ConfigMap for
// mautrix-signal.
func (r *MautrixSignalReconciler) reconcileMautrixSignalConfigMap(ctx context.Context, req ctrl.Request) (*ctrl.Result, error) {
	ms := &synapsev1alpha1.MautrixSignal{}
	if r, err := r.getLatestMautrixSignal(ctx, req, ms); subreconciler.ShouldHaltOrRequeue(r, err) {
		return r, err
	}

	objectMetaMautrixSignal := reconcile.SetObjectMeta(ms.Name, ms.Namespace, map[string]string{})

	desiredConfigMap, err := r.configMapForMautrixSignal(ms, objectMetaMautrixSignal)
	if err != nil {
		return subreconciler.RequeueWithError(err)
	}

	if err := reconcile.ReconcileResource(
		ctx,
		r.Client,
		desiredConfigMap,
		&corev1.ConfigMap{},
	); err != nil {
		return subreconciler.RequeueWithError(err)
	}

	return subreconciler.ContinueReconciling()
}

// configMapForSynapse returns a synapse ConfigMap object
func (r *MautrixSignalReconciler) configMapForMautrixSignal(ms *synapsev1alpha1.MautrixSignal, objectMeta metav1.ObjectMeta) (*corev1.ConfigMap, error) {
	synapseName := ms.Spec.Synapse.Name
	synapseNamespace := utils.ComputeNamespace(ms.Namespace, ms.Spec.Synapse.Namespace)
	synapseServerName := ms.Status.Synapse.ServerName

	configYaml := `
# Homeserver details
homeserver:
    # The address that this appservice can use to connect to the homeserver.
    address: http://` + utils.ComputeFQDN(synapseName, synapseNamespace) + `:8008
    # The domain of the homeserver (for MXIDs, etc).
    domain: ` + synapseServerName + `
    # Whether or not to verify the SSL certificate of the homeserver.
    # Only applies if address starts with https://
    verify_ssl: true
    asmux: false
    # Number of retries for all HTTP requests if the homeserver isn't reachable.
    http_retry_count: 4
    # The URL to push real-time bridge status to.
    # If set, the bridge will make POST requests to this URL whenever a user's Signal connection state changes.
    # The bridge will use the appservice as_token to authorize requests.
    status_endpoint: null
    # Endpoint for reporting per-message status.
    message_send_checkpoint_endpoint: null
    # Maximum number of simultaneous HTTP connections to the homeserver.
    connection_limit: 100
    # Whether asynchronous uploads via MSC2246 should be enabled for media.
    # Requires a media repo that supports MSC2246.
    async_media: false

# Application service host/registration related details
# Changing these values requires regeneration of the registration.
appservice:
    # The address that the homeserver can use to connect to this appservice.
    address: http://` + utils.ComputeFQDN(ms.Name, ms.Namespace) + `:29328
    # When using https:// the TLS certificate and key files for the address.
    tls_cert: false
    tls_key: false

    # The hostname and port where this appservice should listen.
    hostname: 0.0.0.0
    port: 29328
    # The maximum body size of appservice API requests (from the homeserver) in mebibytes
    # Usually 1 is enough, but on high-traffic bridges you might need to increase this to avoid 413s
    max_body_size: 1

    # The full URI to the database. SQLite and Postgres are supported.
    # However, SQLite support is extremely experimental and should not be used.
    # Format examples:
    #   SQLite:   sqlite:///filename.db
    #   Postgres: postgres://username:password@hostname/dbname
    #database: postgres://username:password@hostname/db
    database: sqlite:////data/sqlite.db
    
    # Additional arguments for asyncpg.create_pool() or sqlite3.connect()
    # https://magicstack.github.io/asyncpg/current/api/index.html#asyncpg.pool.create_pool
    # https://docs.python.org/3/library/sqlite3.html#sqlite3.connect
    # For sqlite, min_size is used as the connection thread pool size and max_size is ignored.
    database_opts:
        min_size: 5
        max_size: 10

    # The unique ID of this appservice.
    id: signal
    # Username of the appservice bot.
    bot_username: signalbot
    # Display name and avatar for bot. Set to "remove" to remove display name/avatar, leave empty
    # to leave display name/avatar as-is.
    bot_displayname: Signal bridge bot
    bot_avatar: mxc://maunium.net/wPJgTQbZOtpBFmDNkiNEMDUp

    # Whether or not to receive ephemeral events via appservice transactions.
    # Requires MSC2409 support (i.e. Synapse 1.22+).
    # You should disable bridge -> sync_with_custom_puppets when this is enabled.
    ephemeral_events: false

    # Authentication tokens for AS <-> HS communication. Autogenerated; do not modify.
    as_token: "This value is generated when generating the registration"
    hs_token: "This value is generated when generating the registration"

# Prometheus telemetry config. Requires prometheus-client to be installed.
metrics:
    enabled: false
    listen_port: 8000

# Manhole config.
manhole:
    # Whether or not opening the manhole is allowed.
    enabled: false
    # The path for the unix socket.
    path: /var/tmp/mautrix-signal.manhole
    # The list of UIDs who can be added to the whitelist.
    # If empty, any UIDs can be specified in the open-manhole command.
    whitelist:
    - 0

signal:
    # Path to signald unix socket
    socket_path: /signald/signald.sock
    # Directory for temp files when sending files to Signal. This should be an
    # absolute path that signald can read. For attachments in the other direction,
    # make sure signald is configured to use an absolute path as the data directory.
    outgoing_attachment_dir: /tmp
    # Directory where signald stores avatars for groups.
    avatar_dir: ~/.config/signald/avatars
    # Directory where signald stores auth data. Used to delete data when logging out.
    data_dir: ~/.config/signald/data
    # Whether or not unknown signald accounts should be deleted when the bridge is started.
    # When this is enabled, any UserInUse errors should be resolved by restarting the bridge.
    delete_unknown_accounts_on_start: false
    # Whether or not message attachments should be removed from disk after they're bridged.
    remove_file_after_handling: true
    # Whether or not users can register a primary device
    registration_enabled: true
    # Whether or not to enable disappearing messages in groups. If enabled, then the expiration
    # time of the messages will be determined by the first users to read the message, rather
    # than individually. If the bridge has a single user, this can be turned on safely.
    enable_disappearing_messages_in_groups: false

# Bridge config
bridge:
    # Localpart template of MXIDs for Signal users.
    # {userid} is replaced with an identifier for the Signal user.
    username_template: "signal_{userid}"
    # Displayname template for Signal users.
    # {displayname} is replaced with the displayname of the Signal user, which is the first
    # available variable in displayname_preference. The variables in displayname_preference
    # can also be used here directly.
    displayname_template: "{displayname} (Signal)"
    # Whether or not contact list displaynames should be used.
    # Possible values: disallow, allow, prefer
    #
    # Multi-user instances are recommended to disallow contact list names, as otherwise there can
    # be conflicts between names from different users' contact lists.
    contact_list_names: disallow
    # Available variables: full_name, first_name, last_name, phone, uuid
    displayname_preference:
    - full_name
    - phone

    # Whether or not to create portals for all groups on login/connect.
    autocreate_group_portal: true
    # Whether or not to create portals for all contacts on login/connect.
    autocreate_contact_portal: false
    # Whether or not to use /sync to get read receipts and typing notifications
    # when double puppeting is enabled
    sync_with_custom_puppets: true
    # Whether or not to update the m.direct account data event when double puppeting is enabled.
    # Note that updating the m.direct event is not atomic (except with mautrix-asmux)
    # and is therefore prone to race conditions.
    sync_direct_chat_list: false
    # Allow using double puppeting from any server with a valid client .well-known file.
    double_puppet_allow_discovery: false
    # Servers to allow double puppeting from, even if double_puppet_allow_discovery is false.
    double_puppet_server_map:
        example.com: https://example.com
    # Shared secret for https://github.com/devture/matrix-synapse-shared-secret-auth
    #
    # If set, custom puppets will be enabled automatically for local users
    # instead of users having to find an access token and run 'login-matrix'
    # manually.
    # If using this for other servers than the bridge's server,
    # you must also set the URL in the double_puppet_server_map.
    login_shared_secret_map:
        example.com: foo
    # Whether or not created rooms should have federation enabled.
    # If false, created portal rooms will never be federated.
    federate_rooms: true
    # End-to-bridge encryption support options.
    #
    # See https://docs.mau.fi/bridges/general/end-to-bridge-encryption.html for more info.
    encryption:
        # Allow encryption, work in group chat rooms with e2ee enabled
        allow: false
        # Default to encryption, force-enable encryption in all portals the bridge creates
        # This will cause the bridge bot to be in private chats for the encryption to work properly.
        default: false
        # Options for automatic key sharing.
        key_sharing:
            # Enable key sharing? If enabled, key requests for rooms where users are in will be fulfilled.
            # You must use a client that supports requesting keys from other users to use this feature.
            allow: false
            # Require the requesting device to have a valid cross-signing signature?
            # This doesn't require that the bridge has verified the device, only that the user has verified it.
            # Not yet implemented.
            require_cross_signing: false
            # Require devices to be verified by the bridge?
            # Verification by the bridge is not yet implemented.
            require_verification: true
    # Whether or not to explicitly set the avatar and room name for private
    # chat portal rooms. This will be implicitly enabled if encryption.default is true.
    private_chat_portal_meta: false
    # Whether or not the bridge should send a read receipt from the bridge bot when a message has
    # been sent to Signal. This let's you check manually whether the bridge is receiving your
    # messages.
    # Note that this is not related to Signal delivery receipts.
    delivery_receipts: false
    # Whether or not delivery errors should be reported as messages in the Matrix room. (not yet implemented)
    delivery_error_reports: false
    # Whether the bridge should send the message status as a custom com.beeper.message_send_status event.
    message_status_events: false
    # Set this to true to tell the bridge to re-send m.bridge events to all rooms on the next run.
    # This field will automatically be changed back to false after it,
    # except if the config file is not writable.
    resend_bridge_info: false
    # Interval at which to resync contacts (in seconds).
    periodic_sync: 0
    # Should leaving the room on Matrix make the user leave on Signal?
    bridge_matrix_leave: true

    # Provisioning API part of the web server for automated portal creation and fetching information.
    # Used by things like mautrix-manager (https://github.com/tulir/mautrix-manager).
    provisioning:
        # Whether or not the provisioning API should be enabled.
        enabled: true
        # The prefix to use in the provisioning API endpoints.
        prefix: /_matrix/provision
        # The shared secret to authorize users of the API.
        # Set to "generate" to generate and save a new token.
        shared_secret: generate
        # Segment API key to enable analytics tracking for web server
        # endpoints. Set to null to disable.
        # Currently the only events are login start, QR code scan, and login
        # success/failure.
        segment_key: null

    # The prefix for commands. Only required in non-management rooms.
    command_prefix: "!signal"

    # Messages sent upon joining a management room.
    # Markdown is supported. The defaults are listed below.
    management_room_text:
        # Sent when joining a room.
        welcome: "Hello, I'm a Signal bridge bot."
        # Sent when joining a management room and the user is already logged in.
        welcome_connected: "Use 'help' for help."
        # Sent when joining a management room and the user is not logged in.
        welcome_unconnected: "Use 'help' for help or 'link' to log in."
        # Optional extra text sent when joining a management room.
        additional_help: ""

    # Send each message separately (for readability in some clients)
    management_room_multiple_messages: false

    # Permissions for using the bridge.
    # Permitted values:
    #      relay - Allowed to be relayed through the bridge, no access to commands.
    #       user - Use the bridge with puppeting.
    #      admin - Use and administrate the bridge.
    # Permitted keys:
    #        * - All Matrix users
    #   domain - All users on that homeserver
    #     mxid - Specific user
    permissions:
        "*": "relay"
        "` + synapseServerName + `": "user"
        "@admin:` + synapseServerName + `": "admin"

    relay:
        # Whether relay mode should be allowed. If allowed, '!signal set-relay' can be used to turn any
        # authenticated user into a relaybot for that chat.
        enabled: false
        # The formats to use when sending messages to Signal via a relay user.
        #
        # Available variables:
        #   $sender_displayname - The display name of the sender (e.g. Example User)
        #   $sender_username    - The username (Matrix ID localpart) of the sender (e.g. exampleuser)
        #   $sender_mxid        - The Matrix ID of the sender (e.g. @exampleuser:example.com)
        #   $message            - The message content
        message_formats:
            m.text: '$sender_displayname: $message'
            m.notice: '$sender_displayname: $message'
            m.emote: '* $sender_displayname $message'
            m.file: '$sender_displayname sent a file'
            m.image: '$sender_displayname sent an image'
            m.audio: '$sender_displayname sent an audio file'
            m.video: '$sender_displayname sent a video'
            m.location: '$sender_displayname sent a location'

# Python logging configuration.
#
# See section 16.7.2 of the Python documentation for more info:
# https://docs.python.org/3.6/library/logging.config.html#configuration-dictionary-schema
logging:
    version: 1
    formatters:
        colored:
            (): mautrix_signal.util.ColorFormatter
            format: "[%(asctime)s] [%(levelname)s@%(name)s] %(message)s"
        normal:
            format: "[%(asctime)s] [%(levelname)s@%(name)s] %(message)s"
    handlers:
        file:
            class: logging.handlers.RotatingFileHandler
            formatter: normal
            filename: /data/mautrix-signal.log
            maxBytes: 10485760
            backupCount: 10
        console:
            class: logging.StreamHandler
            formatter: colored
    loggers:
        mau:
            level: DEBUG
        aiohttp:
            level: INFO
    root:
        level: DEBUG
        handlers: [file, console]
`

	cm := &corev1.ConfigMap{
		ObjectMeta: objectMeta,
		Data:       map[string]string{"config.yaml": configYaml},
	}

	// Set Synapse instance as the owner and controller
	if err := ctrl.SetControllerReference(ms, cm, r.Scheme); err != nil {
		return &corev1.ConfigMap{}, err
	}

	return cm, nil
}

// copyInputMautrixSignalConfigMap is a function of type FnWithRequest, to
// be called in the main reconciliation loop.
//
// It creates a copy of the user-provided ConfigMap for mautrix-signal, defined
// in synapse.Spec.Bridges.MautrixSignal.ConfigMap
func (r *MautrixSignalReconciler) copyInputMautrixSignalConfigMap(ctx context.Context, req ctrl.Request) (*ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)

	ms := &synapsev1alpha1.MautrixSignal{}
	if r, err := r.getLatestMautrixSignal(ctx, req, ms); subreconciler.ShouldHaltOrRequeue(r, err) {
		return r, err
	}

	inputConfigMapName := ms.Spec.ConfigMap.Name
	inputConfigMapNamespace := utils.ComputeNamespace(ms.Namespace, ms.Spec.ConfigMap.Namespace)
	keyForInputConfigMap := types.NamespacedName{
		Name:      inputConfigMapName,
		Namespace: inputConfigMapNamespace,
	}

	// Get and check the input ConfigMap for MautrixSignal
	if err := r.Get(ctx, keyForInputConfigMap, &corev1.ConfigMap{}); err != nil {
		reason := "ConfigMap " + inputConfigMapName + " does not exist in namespace " + inputConfigMapNamespace
		ms.Status.State = "FAILED"
		ms.Status.Reason = reason

		err, _ := r.updateMautrixSignalStatus(ctx, ms)
		if err != nil {
			log.Error(err, "Error updating mautrix-signal State")
		}

		log.Error(
			err,
			"Failed to get ConfigMap",
			"ConfigMap.Namespace",
			inputConfigMapNamespace,
			"ConfigMap.Name",
			inputConfigMapName,
		)

		return subreconciler.RequeueWithDelayAndError(time.Duration(30), err)
	}

	objectMetaMautrixSignal := reconcile.SetObjectMeta(ms.Name, ms.Namespace, map[string]string{})

	desiredConfigMap, err := r.configMapForMautrixSignalCopy(ms, objectMetaMautrixSignal)
	if err != nil {
		return subreconciler.RequeueWithError(err)
	}

	// Create a copy of the inputMautrixSignalConfigMap defined in Spec.Bridges.MautrixSignal.ConfigMap
	// Here we use the createdMautrixSignalConfigMap function as createResourceFunc
	if err := reconcile.ReconcileResource(
		ctx,
		r.Client,
		desiredConfigMap,
		&corev1.ConfigMap{},
	); err != nil {
		return subreconciler.RequeueWithError(err)
	}

	return subreconciler.ContinueReconciling()
}

// configMapForMautrixSignalCopy is a function of type createResourceFunc, to be
// passed as an argument in a call to reconcileResouce.
//
// The ConfigMap returned by configMapForMautrixSignalCopy is a copy of the ConfigMap
// defined in Spec.Bridges.MautrixSignal.ConfigMap.
func (r *MautrixSignalReconciler) configMapForMautrixSignalCopy(
	ms *synapsev1alpha1.MautrixSignal,
	objectMeta metav1.ObjectMeta,
) (*corev1.ConfigMap, error) {
	var copyConfigMap *corev1.ConfigMap

	sourceConfigMapName := ms.Spec.ConfigMap.Name
	sourceConfigMapNamespace := utils.ComputeNamespace(ms.Namespace, ms.Spec.ConfigMap.Namespace)

	copyConfigMap, err := utils.GetConfigMapCopy(
		r.Client,
		sourceConfigMapName,
		sourceConfigMapNamespace,
		objectMeta,
	)
	if err != nil {
		return &corev1.ConfigMap{}, err
	}

	// Set Synapse instance as the owner and controller
	if err := ctrl.SetControllerReference(ms, copyConfigMap, r.Scheme); err != nil {
		return &corev1.ConfigMap{}, err
	}

	return copyConfigMap, nil
}

// configureMautrixSignalConfigMap is a function of type FnWithRequest, to
// be called in the main reconciliation loop.
//
// Following the previous copy of the user-provided ConfigMap, it edits the
// content of the copy to ensure that mautrix-signal is correctly configured.
func (r *MautrixSignalReconciler) configureMautrixSignalConfigMap(ctx context.Context, req ctrl.Request) (*ctrl.Result, error) {
	ms := &synapsev1alpha1.MautrixSignal{}
	if r, err := r.getLatestMautrixSignal(ctx, req, ms); subreconciler.ShouldHaltOrRequeue(r, err) {
		return r, err
	}

	keyForConfigMap := types.NamespacedName{
		Name:      ms.Name,
		Namespace: ms.Namespace,
	}

	// Correct data in mautrix-signal ConfigMap
	if err := utils.UpdateConfigMap(
		ctx,
		r.Client,
		keyForConfigMap,
		ms,
		r.updateMautrixSignalData,
		"config.yaml",
	); err != nil {
		return subreconciler.RequeueWithError(err)
	}

	return subreconciler.ContinueReconciling()
}

// updateMautrixSignalData is a function of type updateDataFunc function to
// be passed as an argument in a call to updateConfigMap.
//
// It configures the user-provided config.yaml with the correct values. Among
// other things, it ensures that the bridge can reach the Synapse homeserver
// and knows the correct path to the signald socket.
func (r *MautrixSignalReconciler) updateMautrixSignalData(
	obj client.Object,
	config map[string]interface{},
) error {
	ms := obj.(*synapsev1alpha1.MautrixSignal)

	synapseName := ms.Spec.Synapse.Name
	synapseNamespace := utils.ComputeNamespace(ms.Namespace, ms.Spec.Synapse.Namespace)
	synapseServerName := ms.Status.Synapse.ServerName

	// Update the homeserver section so that the bridge can reach Synapse
	configHomeserver, ok := config["homeserver"].(map[interface{}]interface{})
	if !ok {
		err := errors.New("cannot parse mautrix-signal config.yaml: error parsing 'homeserver' section")
		return err
	}
	configHomeserver["address"] = "http://" + utils.ComputeFQDN(synapseName, synapseNamespace) + ":8008"
	configHomeserver["domain"] = synapseServerName
	config["homeserver"] = configHomeserver

	// Update the appservice section so that Synapse can reach the bridge
	configAppservice, ok := config["appservice"].(map[interface{}]interface{})
	if !ok {
		err := errors.New("cannot parse mautrix-signal config.yaml: error parsing 'appservice' section")
		return err
	}
	configAppservice["address"] = "http://" + utils.ComputeFQDN(ms.Name, ms.Namespace) + ":29328"
	config["appservice"] = configAppservice

	// Update the path to the signal socket path
	configSignal, ok := config["signal"].(map[interface{}]interface{})
	if !ok {
		err := errors.New("cannot parse mautrix-signal config.yaml: error parsing 'signal' section")
		return err
	}
	configSignal["socket_path"] = "/signald/signald.sock"
	config["signal"] = configSignal

	// Update persmissions to use the correct domain name
	configBridge, ok := config["bridge"].(map[interface{}]interface{})
	if !ok {
		err := errors.New("cannot parse mautrix-signal config.yaml: error parsing 'bridge' section")
		return err
	}
	configBridge["permissions"] = map[string]string{
		"*":                           "relay",
		synapseServerName:             "user",
		"@admin:" + synapseServerName: "admin",
	}
	config["bridge"] = configBridge

	// Update the path to the log file
	configLogging, ok := config["logging"].(map[interface{}]interface{})
	if !ok {
		err := errors.New("cannot parse mautrix-signal config.yaml: error parsing 'logging' section")
		return err
	}
	configLoggingHandlers, ok := configLogging["handlers"].(map[interface{}]interface{})
	if !ok {
		err := errors.New("cannot parse mautrix-signal config.yaml: error parsing 'logging/handlers' section")
		return err
	}
	configLoggingHandlersFile, ok := configLoggingHandlers["file"].(map[interface{}]interface{})
	if !ok {
		err := errors.New("cannot parse mautrix-signal config.yaml: error parsing 'logging/handlers/file' section")
		return err
	}
	configLoggingHandlersFile["filename"] = "/data/mautrix-signal.log"
	configLoggingHandlers["file"] = configLoggingHandlersFile
	configLogging["handlers"] = configLoggingHandlers
	config["logging"] = configLogging

	return nil
}
