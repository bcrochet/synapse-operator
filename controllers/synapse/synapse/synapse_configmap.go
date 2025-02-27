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

package synapse

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	subreconciler "github.com/opdev/subreconciler"
	synapsev1alpha1 "github.com/opdev/synapse-operator/apis/synapse/v1alpha1"
	"github.com/opdev/synapse-operator/helpers/reconcile"
	"github.com/opdev/synapse-operator/helpers/utils"
)

// reconcileSynapseConfigMap is a function of type FnWithRequest, to be
// called in the main reconciliation loop.
//
// It reconciles the synapse ConfigMap to its desired state. It is called only
// if the user hasn't provided its own ConfigMap for synapse
func (r *SynapseReconciler) reconcileSynapseConfigMap(ctx context.Context, req ctrl.Request) (*ctrl.Result, error) {
	s := &synapsev1alpha1.Synapse{}
	if r, err := r.getLatestSynapse(ctx, req, s); subreconciler.ShouldHaltOrRequeue(r, err) {
		return r, err
	}

	objectMetaForSynapse := reconcile.SetObjectMeta(s.Name, s.Namespace, map[string]string{})

	desiredConfigMap, err := r.configMapForSynapse(s, objectMetaForSynapse)
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
func (r *SynapseReconciler) configMapForSynapse(s *synapsev1alpha1.Synapse, objectMeta metav1.ObjectMeta) (*corev1.ConfigMap, error) {
	homeserverYaml := `
# Configuration file for Synapse.
#
# This is a YAML file: see [1] for a quick introduction. Note in particular
# that *indentation is important*: all the elements of a list or dictionary
# should have the same indentation.
#
# [1] https://docs.ansible.com/ansible/latest/reference_appendices/YAMLSyntax.html

## Server ##

# The public-facing domain of the server
#
# The server_name name will appear at the end of usernames and room addresses
# created on this server. For example if the server_name was example.com,
# usernames on this server would be in the format @user:example.com
#
# In most cases you should avoid using a matrix specific subdomain such as
# matrix.example.com or synapse.example.com as the server_name for the same
# reasons you wouldn't use user@email.example.com as your email address.
# See https://github.com/matrix-org/synapse/blob/master/docs/delegate.md
# for information on how to host Synapse on a subdomain while preserving
# a clean server_name.
#
# The server_name cannot be changed later so it is important to
# configure this correctly before you start Synapse. It should be all
# lowercase and may contain an explicit port.
# Examples: matrix.org, localhost:8080
#
server_name: "` + s.Spec.Homeserver.Values.ServerName + `"

# When running as a daemon, the file to store the pid in
#
pid_file: /homeserver.pid

# The absolute URL to the web client which /_matrix/client will redirect
# to if 'webclient' is configured under the 'listeners' configuration.
#
# This option can be also set to the filesystem path to the web client
# which will be served at /_matrix/client/ if 'webclient' is configured
# under the 'listeners' configuration, however this is a security risk:
# https://github.com/matrix-org/synapse#security-note
#
#web_client_location: https://riot.example.com/

# The public-facing base URL that clients use to access this HS
# (not including _matrix/...). This is the same URL a user would
# enter into the 'custom HS URL' field on their client. If you
# use synapse with a reverse proxy, this should be the URL to reach
# synapse via the proxy.
#
#public_baseurl: https://example.com/

# Set the soft limit on the number of file descriptors synapse can use
# Zero is used to indicate synapse should set the soft limit to the
# hard limit.
#
#soft_file_limit: 0

# Set to false to disable presence tracking on this homeserver.
#
#use_presence: false

# Whether to require authentication to retrieve profile data (avatars,
# display names) of other users through the client API. Defaults to
# 'false'. Note that profile data is also available via the federation
# API, so this setting is of limited value if federation is enabled on
# the server.
#
#require_auth_for_profile_requests: true

# Uncomment to require a user to share a room with another user in order
# to retrieve their profile information. Only checked on Client-Server
# requests. Profile requests from other servers should be checked by the
# requesting server. Defaults to 'false'.
#
#limit_profile_requests_to_users_who_share_rooms: true

# If set to 'true', removes the need for authentication to access the server's
# public rooms directory through the client API, meaning that anyone can
# query the room directory. Defaults to 'false'.
#
#allow_public_rooms_without_auth: true

# If set to 'true', allows any other homeserver to fetch the server's public
# rooms directory via federation. Defaults to 'false'.
#
#allow_public_rooms_over_federation: true

# The default room version for newly created rooms.
#
# Known room versions are listed here:
# https://matrix.org/docs/spec/#complete-list-of-room-versions
#
# For example, for room version 1, default_room_version should be set
# to "1".
#
#default_room_version: "6"

# The GC threshold parameters to pass to 'gc.set_threshold ', if defined
#
#gc_thresholds: [700, 10, 10]

# Set the limit on the returned events in the timeline in the get
# and sync operations. The default value is 100. -1 means no upper limit.
#
# Uncomment the following to increase the limit to 5000.
#
#filter_timeline_limit: 5000

# Whether room invites to users on this server should be blocked
# (except those sent by local server admins). The default is False.
#
#block_non_admin_invites: true

# Room searching
#
# If disabled, new messages will not be indexed for searching and users
# will receive errors when searching for messages. Defaults to enabled.
#
#enable_search: false

# Prevent outgoing requests from being sent to the following blacklisted IP address
# CIDR ranges. If this option is not specified then it defaults to private IP
# address ranges (see the example below).
#
# The blacklist applies to the outbound requests for federation, identity servers,
# push servers, and for checking key validity for third-party invite events.
#
# (0.0.0.0 and :: are always blacklisted, whether or not they are explicitly
# listed here, since they correspond to unroutable addresses.)
#
# This option replaces federation_ip_range_blacklist in Synapse v1.25.0.
#
#ip_range_blacklist:
#  - '127.0.0.0/8'
#  - '10.0.0.0/8'
#  - '172.16.0.0/12'
#  - '192.168.0.0/16'
#  - '100.64.0.0/10'
#  - '192.0.0.0/24'
#  - '169.254.0.0/16'
#  - '198.18.0.0/15'
#  - '192.0.2.0/24'
#  - '198.51.100.0/24'
#  - '203.0.113.0/24'
#  - '224.0.0.0/4'
#  - '::1/128'
#  - 'fe80::/10'
#  - 'fc00::/7'

# List of IP address CIDR ranges that should be allowed for federation,
# identity servers, push servers, and for checking key validity for
# third-party invite events. This is useful for specifying exceptions to
# wide-ranging blacklisted target IP ranges - e.g. for communication with
# a push server only visible in your network.
#
# This whitelist overrides ip_range_blacklist and defaults to an empty
# list.
#
#ip_range_whitelist:
#   - '192.168.1.1'

# List of ports that Synapse should listen on, their purpose and their
# configuration.
#
# Options for each listener include:
#
#   port: the TCP port to bind to
#
#   bind_addresses: a list of local addresses to listen on. The default is
#       'all local interfaces'.
#
#   type: the type of listener. Normally 'http', but other valid options are:
#       'manhole' (see docs/manhole.md),
#       'metrics' (see docs/metrics-howto.md),
#       'replication' (see docs/workers.md).
#
#   tls: set to true to enable TLS for this listener. Will use the TLS
#       key/cert specified in tls_private_key_path / tls_certificate_path.
#
#   x_forwarded: Only valid for an 'http' listener. Set to true to use the
#       X-Forwarded-For header as the client IP. Useful when Synapse is
#       behind a reverse-proxy.
#
#   resources: Only valid for an 'http' listener. A list of resources to host
#       on this port. Options for each resource are:
#
#       names: a list of names of HTTP resources. See below for a list of
#           valid resource names.
#
#       compress: set to true to enable HTTP compression for this resource.
#
#   additional_resources: Only valid for an 'http' listener. A map of
#        additional endpoints which should be loaded via dynamic modules.
#
# Valid resource names are:
#
#   client: the client-server API (/_matrix/client), and the synapse admin
#       API (/_synapse/admin). Also implies 'media' and 'static'.
#
#   consent: user consent forms (/_matrix/consent). See
#       docs/consent_tracking.md.
#
#   federation: the server-server API (/_matrix/federation). Also implies
#       'media', 'keys', 'openid'
#
#   keys: the key discovery API (/_matrix/keys).
#
#   media: the media API (/_matrix/media).
#
#   metrics: the metrics interface. See docs/metrics-howto.md.
#
#   openid: OpenID authentication.
#
#   replication: the HTTP replication API (/_synapse/replication). See
#       docs/workers.md.
#
#   static: static resources under synapse/static (/_matrix/static). (Mostly
#       useful for 'fallback authentication'.)
#
#   webclient: A web client. Requires web_client_location to be set.
#
listeners:
  # TLS-enabled listener: for when matrix traffic is sent directly to synapse.
  #
  # Disabled by default. To enable it, uncomment the following. (Note that you
  # will also need to give Synapse a TLS key and certificate: see the TLS section
  # below.)
  #
  #- port: 8448
  #  type: http
  #  tls: true
  #  resources:
  #    - names: [client, federation]

  # Unsecure HTTP listener: for when matrix traffic passes through a reverse proxy
  # that unwraps TLS.
  #
  # If you plan to use a reverse proxy, please see
  # https://github.com/matrix-org/synapse/blob/master/docs/reverse_proxy.md.
  #
  - port: 8008
    tls: false
    type: http
    x_forwarded: true

    resources:
      - names: [client, federation]
        compress: false

    # example additional_resources:
    #
    #additional_resources:
    #  "/_matrix/my/custom/endpoint":
    #    module: my_module.CustomRequestHandler
    #    config: {}

  # Turn on the twisted ssh manhole service on localhost on the given
  # port.
  #
  #- port: 9000
  #  bind_addresses: ['::1', '127.0.0.1']
  #  type: manhole

  # Forward extremities can build up in a room due to networking delays between
# homeservers. Once this happens in a large room, calculation of the state of
# that room can become quite expensive. To mitigate this, once the number of
# forward extremities reaches a given threshold, Synapse will send an
# org.matrix.dummy_event event, which will reduce the forward extremities
# in the room.
#
# This setting defines the threshold (i.e. number of forward extremities in the
# room) at which dummy events are sent. The default value is 10.
#
#dummy_events_threshold: 5


## Homeserver blocking ##

# How to reach the server admin, used in ResourceLimitError
#
#admin_contact: 'mailto:admin@server.com'

# Global blocking
#
#hs_disabled: false
#hs_disabled_message: 'Human readable reason for why the HS is blocked'

# Monthly Active User Blocking
#
# Used in cases where the admin or server owner wants to limit to the
# number of monthly active users.
#
# 'limit_usage_by_mau' disables/enables monthly active user blocking. When
# enabled and a limit is reached the server returns a 'ResourceLimitError'
# with error type Codes.RESOURCE_LIMIT_EXCEEDED
#
# 'max_mau_value' is the hard limit of monthly active users above which
# the server will start blocking user actions.
#
# 'mau_trial_days' is a means to add a grace period for active users. It
# means that users must be active for this number of days before they
# can be considered active and guards against the case where lots of users
# sign up in a short space of time never to return after their initial
# session.
#
# 'mau_limit_alerting' is a means of limiting client side alerting
# should the mau limit be reached. This is useful for small instances
# where the admin has 5 mau seats (say) for 5 specific people and no
# interest increasing the mau limit further. Defaults to True, which
# means that alerting is enabled
#
#limit_usage_by_mau: false
#max_mau_value: 50
#mau_trial_days: 2
#mau_limit_alerting: false

# If enabled, the metrics for the number of monthly active users will
# be populated, however no one will be limited. If limit_usage_by_mau
# is true, this is implied to be true.
#
#mau_stats_only: false

# Sometimes the server admin will want to ensure certain accounts are
# never blocked by mau checking. These accounts are specified here.
#
#mau_limit_reserved_threepids:
#  - medium: 'email'
#    address: 'reserved_user@example.com'

# Used by phonehome stats to group together related servers.
#server_context: context

# Resource-constrained homeserver settings
#
# When this is enabled, the room "complexity" will be checked before a user
# joins a new remote room. If it is above the complexity limit, the server will
# disallow joining, or will instantly leave.
#
# Room complexity is an arbitrary measure based on factors such as the number of
# users in the room.
#
limit_remote_rooms:
  # Uncomment to enable room complexity checking.
  #
  #enabled: true

  # the limit above which rooms cannot be joined. The default is 1.0.
  #
  #complexity: 0.5

  # override the error which is returned when the room is too complex.
  #
  #complexity_error: "This room is too complex."

  # allow server admins to join complex rooms. Default is false.
  #
  #admins_can_join: true

  # Whether to require a user to be in the room to add an alias to it.
# Defaults to 'true'.
#
#require_membership_for_aliases: false

# Whether to allow per-room membership profiles through the send of membership
# events with profile information that differ from the target's global profile.
# Defaults to 'true'.
#
#allow_per_room_profiles: false

# How long to keep redacted events in unredacted form in the database. After
# this period redacted events get replaced with their redacted form in the DB.
#
# Defaults to  '7d '. Set to  'null ' to disable.
#
#redaction_retention_period: 28d

# How long to track users' last seen time and IPs in the database.
#
# Defaults to  '28d '. Set to  'null ' to disable clearing out of old rows.
#
#user_ips_max_age: 14d

# Message retention policy at the server level.
#
# Room admins and mods can define a retention period for their rooms using the
# 'm.room.retention' state event, and server admins can cap this period by setting
# the 'allowed_lifetime_min' and 'allowed_lifetime_max' config options.
#
# If this feature is enabled, Synapse will regularly look for and purge events
# which are older than the room's maximum retention period. Synapse will also
# filter events received over federation so that events that should have been
# purged are ignored and not stored again.
#
retention:
  # The message retention policies feature is disabled by default. Uncomment the
  # following line to enable it.
  #
  #enabled: true

  # Default retention policy. If set, Synapse will apply it to rooms that lack the
  # 'm.room.retention' state event. Currently, the value of 'min_lifetime' doesn't
  # matter much because Synapse doesn't take it into account yet.
  #
  #default_policy:
  #  min_lifetime: 1d
  #  max_lifetime: 1y

  # Retention policy limits. If set, and the state of a room contains a
  # 'm.room.retention' event in its state which contains a 'min_lifetime' or a
  # 'max_lifetime' that's out of these bounds, Synapse will cap the room's policy
  # to these limits when running purge jobs.
  #
  #allowed_lifetime_min: 1d
  #allowed_lifetime_max: 1y

  # Server admins can define the settings of the background jobs purging the
  # events which lifetime has expired under the 'purge_jobs' section.
  #
  # If no configuration is provided, a single job will be set up to delete expired
  # events in every room daily.
  #
  # Each job's configuration defines which range of message lifetimes the job
  # takes care of. For example, if 'shortest_max_lifetime' is '2d' and
  # 'longest_max_lifetime' is '3d', the job will handle purging expired events in
  # rooms whose state defines a 'max_lifetime' that's both higher than 2 days, and
  # lower than or equal to 3 days. Both the minimum and the maximum value of a
  # range are optional, e.g. a job with no 'shortest_max_lifetime' and a
  # 'longest_max_lifetime' of '3d' will handle every room with a retention policy
  # which 'max_lifetime' is lower than or equal to three days.
  #
  # The rationale for this per-job configuration is that some rooms might have a
  # retention policy with a low 'max_lifetime', where history needs to be purged
  # of outdated messages on a more frequent basis than for the rest of the rooms
  # (e.g. every 12h), but not want that purge to be performed by a job that's
  # iterating over every room it knows, which could be heavy on the server.
  #
  # If any purge job is configured, it is strongly recommended to have at least
  # a single job with neither 'shortest_max_lifetime' nor 'longest_max_lifetime'
  # set, or one job without 'shortest_max_lifetime' and one job without
  # 'longest_max_lifetime' set. Otherwise some rooms might be ignored, even if
  # 'allowed_lifetime_min' and 'allowed_lifetime_max' are set, because capping a
  # room's policy to these values is done after the policies are retrieved from
  # Synapse's database (which is done using the range specified in a purge job's
  # configuration).
  #
  #purge_jobs:
  #  - longest_max_lifetime: 3d
  #    interval: 12h
  #  - shortest_max_lifetime: 3d
  #    interval: 1d

  # Inhibits the /requestToken endpoints from returning an error that might leak
# information about whether an e-mail address is in use or not on this
# homeserver.
# Note that for some endpoints the error situation is the e-mail already being
# used, and for others the error is entering the e-mail being unused.
# If this option is enabled, instead of returning an error, these endpoints will
# act as if no error happened and return a fake session ID ('sid') to clients.
#
#request_token_inhibit_3pid_errors: true

# A list of domains that the domain portion of 'next_link' parameters
# must match.
#
# This parameter is optionally provided by clients while requesting
# validation of an email or phone number, and maps to a link that
# users will be automatically redirected to after validation
# succeeds. Clients can make use this parameter to aid the validation
# process.
#
# The whitelist is applied whether the homeserver or an
# identity server is handling validation.
#
# The default value is no whitelist functionality; all domains are
# allowed. Setting this value to an empty list will instead disallow
# all domains.
#
#next_link_domain_whitelist: ["matrix.org"]


## TLS ##

# PEM-encoded X509 certificate for TLS.
# This certificate, as of Synapse 1.0, will need to be a valid and verifiable
# certificate, signed by a recognised Certificate Authority.
#
# See 'ACME support' below to enable auto-provisioning this certificate via
# Let's Encrypt.
#
# If supplying your own, be sure to use a  '.pem ' file that includes the
# full certificate chain including any intermediate certificates (for
# instance, if using certbot, use  'fullchain.pem ' as your certificate,
# not  'cert.pem ').
#
#tls_certificate_path: "/example.com.tls.crt"

# PEM-encoded private key for TLS
#
#tls_private_key_path: "/example.com.tls.key"

# Whether to verify TLS server certificates for outbound federation requests.
#
# Defaults to  'true '. To disable certificate verification, uncomment the
# following line.
#
#federation_verify_certificates: false

# The minimum TLS version that will be used for outbound federation requests.
#
# Defaults to  '1 '. Configurable to  '1 ',  '1.1 ',  '1.2 ', or  '1.3 '. Note
# that setting this value higher than  '1.2 ' will prevent federation to most
# of the public Matrix network: only configure it to  '1.3 ' if you have an
# entirely private federation setup and you can ensure TLS 1.3 support.
#
#federation_client_minimum_tls_version: 1.2

# Skip federation certificate verification on the following whitelist
# of domains.
#
# This setting should only be used in very specific cases, such as
# federation over Tor hidden services and similar. For private networks
# of homeservers, you likely want to use a private CA instead.
#
# Only effective if federation_verify_certicates is  'true '.
#
#federation_certificate_verification_whitelist:
#  - lon.example.com
#  - *.domain.com
#  - *.onion

# List of custom certificate authorities for federation traffic.
#
# This setting should only normally be used within a private network of
# homeservers.
#
# Note that this list will replace those that are provided by your
# operating environment. Certificates must be in PEM format.
#
#federation_custom_ca_list:
#  - myCA1.pem
#  - myCA2.pem
#  - myCA3.pem

# ACME support: This will configure Synapse to request a valid TLS certificate
# for your configured  'server_name ' via Let's Encrypt.
#
# Note that ACME v1 is now deprecated, and Synapse currently doesn't support
# ACME v2. This means that this feature currently won't work with installs set
# up after November 2019. For more info, and alternative solutions, see
# https://github.com/matrix-org/synapse/blob/master/docs/ACME.md#deprecation-of-acme-v1
#
# Note that provisioning a certificate in this way requires port 80 to be
# routed to Synapse so that it can complete the http-01 ACME challenge.
# By default, if you enable ACME support, Synapse will attempt to listen on
# port 80 for incoming http-01 challenges - however, this will likely fail
# with 'Permission denied' or a similar error.
#
# There are a couple of potential solutions to this:
#
#  * If you already have an Apache, Nginx, or similar listening on port 80,
#    you can configure Synapse to use an alternate port, and have your web
#    server forward the requests. For example, assuming you set 'port: 8009'
#    below, on Apache, you would write:
#
#    ProxyPass /.well-known/acme-challenge http://localhost:8009/.well-known/acme-challenge
#
#  * Alternatively, you can use something like  'authbind ' to give Synapse
#    permission to listen on port 80.
#
acme:
    # ACME support is disabled by default. Set this to  'true ' and uncomment
    # tls_certificate_path and tls_private_key_path above to enable it.
    #
    enabled: false

    # Endpoint to use to request certificates. If you only want to test,
    # use Let's Encrypt's staging url:
    #     https://acme-staging.api.letsencrypt.org/directory
    #
    #url: https://acme-v01.api.letsencrypt.org/directory

    # Port number to listen on for the HTTP-01 challenge. Change this if
    # you are forwarding connections through Apache/Nginx/etc.
    #
    port: 80

    # Local addresses to listen on for incoming connections.
    # Again, you may want to change this if you are forwarding connections
    # through Apache/Nginx/etc.
    #
    bind_addresses: ['::', '0.0.0.0']

    # How many days remaining on a certificate before it is renewed.
    #
    reprovision_threshold: 30

    # The domain that the certificate should be for. Normally this
    # should be the same as your Matrix domain (i.e., 'server_name'), but,
    # by putting a file at 'https://<server_name>/.well-known/matrix/server',
    # you can delegate incoming traffic to another server. If you do that,
    # you should give the target of the delegation here.
    #
    # For example: if your 'server_name' is 'example.com', but
    # 'https://example.com/.well-known/matrix/server' delegates to
    # 'matrix.example.com', you should put 'matrix.example.com' here.
    #
    # If not set, defaults to your 'server_name'.
    #
    domain: matrix.example.com

    # file to use for the account key. This will be generated if it doesn't
    # exist.
    #
    # If unspecified, we will use CONFDIR/client.key.
    #
    account_key_file: /acme_account.key

    # List of allowed TLS fingerprints for this server to publish along
# with the signing keys for this server. Other matrix servers that
# make HTTPS requests to this server will check that the TLS
# certificates returned by this server match one of the fingerprints.
#
# Synapse automatically adds the fingerprint of its own certificate
# to the list. So if federation traffic is handled directly by synapse
# then no modification to the list is required.
#
# If synapse is run behind a load balancer that handles the TLS then it
# will be necessary to add the fingerprints of the certificates used by
# the loadbalancers to this list if they are different to the one
# synapse is using.
#
# Homeservers are permitted to cache the list of TLS fingerprints
# returned in the key responses up to the "valid_until_ts" returned in
# key. It may be necessary to publish the fingerprints of a new
# certificate and wait until the "valid_until_ts" of the previous key
# responses have passed before deploying it.
#
# You can calculate a fingerprint from a given TLS listener via:
# openssl s_client -connect $host:$port < /dev/null 2> /dev/null |
#   openssl x509 -outform DER | openssl sha256 -binary | base64 | tr -d '='
# or by checking matrix.org/federationtester/api/report?server_name=$host
#
#tls_fingerprints: [{"sha256": "<base64_encoded_sha256_fingerprint>"}]


## Federation ##

# Restrict federation to the following whitelist of domains.
# N.B. we recommend also firewalling your federation listener to limit
# inbound federation traffic as early as possible, rather than relying
# purely on this application-layer restriction.  If not specified, the
# default is to whitelist everything.
#
#federation_domain_whitelist:
#  - lon.example.com
#  - nyc.example.com
#  - syd.example.com

# Report prometheus metrics on the age of PDUs being sent to and received from
# the following domains. This can be used to give an idea of "delay" on inbound
# and outbound federation, though be aware that any delay can be due to problems
# at either end or with the intermediate network.
#
# By default, no domains are monitored in this way.
#
#federation_metrics_domains:
#  - matrix.org
#  - example.com


## Caching ##

# Caching can be configured through the following options.
#
# A cache 'factor' is a multiplier that can be applied to each of
# Synapse's caches in order to increase or decrease the maximum
# number of entries that can be stored.

# The number of events to cache in memory. Not affected by
# caches.global_factor.
#
#event_cache_size: 10K

caches:
   # Controls the global cache factor, which is the default cache factor
   # for all caches if a specific factor for that cache is not otherwise
   # set.
   #
   # This can also be set by the "SYNAPSE_CACHE_FACTOR" environment
   # variable. Setting by environment variable takes priority over
   # setting through the config file.
   #
   # Defaults to 0.5, which will half the size of all caches.
   #
   #global_factor: 1.0

   # A dictionary of cache name to cache factor for that individual
   # cache. Overrides the global cache factor for a given cache.
   #
   # These can also be set through environment variables comprised
   # of "SYNAPSE_CACHE_FACTOR_" + the name of the cache in capital
   # letters and underscores. Setting by environment variable
   # takes priority over setting through the config file.
   # Ex. SYNAPSE_CACHE_FACTOR_GET_USERS_WHO_SHARE_ROOM_WITH_USER=2.0
   #
   # Some caches have '*' and other characters that are not
   # alphanumeric or underscores. These caches can be named with or
   # without the special characters stripped. For example, to specify
   # the cache factor for  '*stateGroupCache* ' via an environment
   # variable would be  'SYNAPSE_CACHE_FACTOR_STATEGROUPCACHE=2.0 '.
   #
   per_cache_factors:
     #get_users_who_share_room_with_user: 2.0


     ## Database ##

# The 'database' setting defines the database that synapse uses to store all of
# its data.
#
# 'name' gives the database engine to use: either 'sqlite3' (for SQLite) or
# 'psycopg2' (for PostgreSQL).
#
# 'args' gives options which are passed through to the database engine,
# except for options starting 'cp_', which are used to configure the Twisted
# connection pool. For a reference to valid arguments, see:
#   * for sqlite: https://docs.python.org/3/library/sqlite3.html#sqlite3.connect
#   * for postgres: https://www.postgresql.org/docs/current/libpq-connect.html#LIBPQ-PARAMKEYWORDS
#   * for the connection pool: https://twistedmatrix.com/documents/current/api/twisted.enterprise.adbapi.ConnectionPool.html#__init__
#
#
# Example SQLite configuration:
#
#database:
#  name: sqlite3
#  args:
#    database: /path/to/homeserver.db
#
#
# Example Postgres configuration:
#
#database:
#  name: psycopg2
#  args:
#    user: synapse_user
#    password: secretpassword
#    database: synapse
#    host: localhost
#    cp_min: 5
#    cp_max: 10
#
# For more information on using Synapse with Postgres, see  'docs/postgres.md '.
#
database:
  name: sqlite3
  args:
    database: /data/homeserver.db


    ## Logging ##

# A yaml python logging config file as described by
# https://docs.python.org/3.7/library/logging.config.html#configuration-dictionary-schema
#
log_config: "/data/` + s.Spec.Homeserver.Values.ServerName + `.log.config"


## Ratelimiting ##

# Ratelimiting settings for client actions (registration, login, messaging).
#
# Each ratelimiting configuration is made of two parameters:
#   - per_second: number of requests a client can send per second.
#   - burst_count: number of requests a client can send before being throttled.
#
# Synapse currently uses the following configurations:
#   - one for messages that ratelimits sending based on the account the client
#     is using
#   - one for registration that ratelimits registration requests based on the
#     client's IP address.
#   - one for login that ratelimits login requests based on the client's IP
#     address.
#   - one for login that ratelimits login requests based on the account the
#     client is attempting to log into.
#   - one for login that ratelimits login requests based on the account the
#     client is attempting to log into, based on the amount of failed login
#     attempts for this account.
#   - one for ratelimiting redactions by room admins. If this is not explicitly
#     set then it uses the same ratelimiting as per rc_message. This is useful
#     to allow room admins to deal with abuse quickly.
#   - two for ratelimiting number of rooms a user can join, "local" for when
#     users are joining rooms the server is already in (this is cheap) vs
#     "remote" for when users are trying to join rooms not on the server (which
#     can be more expensive)
#
# The defaults are as shown below.
#
#rc_message:
#  per_second: 0.2
#  burst_count: 10
#
#rc_registration:
#  per_second: 0.17
#  burst_count: 3
#
#rc_login:
#  address:
#    per_second: 0.17
#    burst_count: 3
#  account:
#    per_second: 0.17
#    burst_count: 3
#  failed_attempts:
#    per_second: 0.17
#    burst_count: 3
#
#rc_admin_redaction:
#  per_second: 1
#  burst_count: 50
#
#rc_joins:
#  local:
#    per_second: 0.1
#    burst_count: 3
#  remote:
#    per_second: 0.01
#    burst_count: 3


# Ratelimiting settings for incoming federation
#
# The rc_federation configuration is made up of the following settings:
#   - window_size: window size in milliseconds
#   - sleep_limit: number of federation requests from a single server in
#     a window before the server will delay processing the request.
#   - sleep_delay: duration in milliseconds to delay processing events
#     from remote servers by if they go over the sleep limit.
#   - reject_limit: maximum number of concurrent federation requests
#     allowed from a single server
#   - concurrent: number of federation requests to concurrently process
#     from a single server
#
# The defaults are as shown below.
#
#rc_federation:
#  window_size: 1000
#  sleep_limit: 10
#  sleep_delay: 500
#  reject_limit: 50
#  concurrent: 3

# Target outgoing federation transaction frequency for sending read-receipts,
# per-room.
#
# If we end up trying to send out more read-receipts, they will get buffered up
# into fewer transactions.
#
#federation_rr_transactions_per_room_per_second: 50



## Media Store ##

# Enable the media store service in the Synapse master. Uncomment the
# following if you are using a separate media store worker.
#
#enable_media_repo: false

# Directory where uploaded images and attachments are stored.
#
media_store_path: "/data/media_store"

# Media storage providers allow media to be stored in different
# locations.
#
#media_storage_providers:
#  - module: file_system
#    # Whether to store newly uploaded local files
#    store_local: false
#    # Whether to store newly downloaded remote files
#    store_remote: false
#    # Whether to wait for successful storage for local uploads
#    store_synchronous: false
#    config:
#       directory: /mnt/some/other/directory

# The largest allowed upload size in bytes
#
#max_upload_size: 50M

# Maximum number of pixels that will be thumbnailed
#
#max_image_pixels: 32M

# Whether to generate new thumbnails on the fly to precisely match
# the resolution requested by the client. If true then whenever
# a new resolution is requested by the client the server will
# generate a new thumbnail. If false the server will pick a thumbnail
# from a precalculated list.
#
#dynamic_thumbnails: false

# List of thumbnails to precalculate when an image is uploaded.
#
#thumbnail_sizes:
#  - width: 32
#    height: 32
#    method: crop
#  - width: 96
#    height: 96
#    method: crop
#  - width: 320
#    height: 240
#    method: scale
#  - width: 640
#    height: 480
#    method: scale
#  - width: 800
#    height: 600
#    method: scale

# Is the preview URL API enabled?
#
# 'false' by default: uncomment the following to enable it (and specify a
# url_preview_ip_range_blacklist blacklist).
#
#url_preview_enabled: true

# List of IP address CIDR ranges that the URL preview spider is denied
# from accessing.  There are no defaults: you must explicitly
# specify a list for URL previewing to work.  You should specify any
# internal services in your network that you do not want synapse to try
# to connect to, otherwise anyone in any Matrix room could cause your
# synapse to issue arbitrary GET requests to your internal services,
# causing serious security issues.
#
# (0.0.0.0 and :: are always blacklisted, whether or not they are explicitly
# listed here, since they correspond to unroutable addresses.)
#
# This must be specified if url_preview_enabled is set. It is recommended that
# you uncomment the following list as a starting point.
#
#url_preview_ip_range_blacklist:
#  - '127.0.0.0/8'
#  - '10.0.0.0/8'
#  - '172.16.0.0/12'
#  - '192.168.0.0/16'
#  - '100.64.0.0/10'
#  - '192.0.0.0/24'
#  - '169.254.0.0/16'
#  - '198.18.0.0/15'
#  - '192.0.2.0/24'
#  - '198.51.100.0/24'
#  - '203.0.113.0/24'
#  - '224.0.0.0/4'
#  - '::1/128'
#  - 'fe80::/10'
#  - 'fc00::/7'

# List of IP address CIDR ranges that the URL preview spider is allowed
# to access even if they are specified in url_preview_ip_range_blacklist.
# This is useful for specifying exceptions to wide-ranging blacklisted
# target IP ranges - e.g. for enabling URL previews for a specific private
# website only visible in your network.
#
#url_preview_ip_range_whitelist:
#   - '192.168.1.1'

# Optional list of URL matches that the URL preview spider is
# denied from accessing.  You should use url_preview_ip_range_blacklist
# in preference to this, otherwise someone could define a public DNS
# entry that points to a private IP address and circumvent the blacklist.
# This is more useful if you know there is an entire shape of URL that
# you know that will never want synapse to try to spider.
#
# Each list entry is a dictionary of url component attributes as returned
# by urlparse.urlsplit as applied to the absolute form of the URL.  See
# https://docs.python.org/2/library/urlparse.html#urlparse.urlsplit
# The values of the dictionary are treated as an filename match pattern
# applied to that component of URLs, unless they start with a ^ in which
# case they are treated as a regular expression match.  If all the
# specified component matches for a given list item succeed, the URL is
# blacklisted.
#
#url_preview_url_blacklist:
#  # blacklist any URL with a username in its URI
#  - username: '*'
#
#  # blacklist all *.google.com URLs
#  - netloc: 'google.com'
#  - netloc: '*.google.com'
#
#  # blacklist all plain HTTP URLs
#  - scheme: 'http'
#
#  # blacklist http(s)://www.acme.com/foo
#  - netloc: 'www.acme.com'
#    path: '/foo'
#
#  # blacklist any URL with a literal IPv4 address
#  - netloc: '^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$'

# The largest allowed URL preview spidering size in bytes
#
#max_spider_size: 10M

# A list of values for the Accept-Language HTTP header used when
# downloading webpages during URL preview generation. This allows
# Synapse to specify the preferred languages that URL previews should
# be in when communicating with remote servers.
#
# Each value is a IETF language tag; a 2-3 letter identifier for a
# language, optionally followed by subtags separated by '-', specifying
# a country or region variant.
#
# Multiple values can be provided, and a weight can be added to each by
# using quality value syntax (;q=). '*' translates to any language.
#
# Defaults to "en".
#
# Example:
#
# url_preview_accept_language:
#   - en-UK
#   - en-US;q=0.9
#   - fr;q=0.8
#   - *;q=0.7
#
url_preview_accept_language:
#   - en


## Captcha ##
# See docs/CAPTCHA_SETUP.md for full details of configuring this.

# This homeserver's ReCAPTCHA public key. Must be specified if
# enable_registration_captcha is enabled.
#
#recaptcha_public_key: "YOUR_PUBLIC_KEY"

# This homeserver's ReCAPTCHA private key. Must be specified if
# enable_registration_captcha is enabled.
#
#recaptcha_private_key: "YOUR_PRIVATE_KEY"

# Uncomment to enable ReCaptcha checks when registering, preventing signup
# unless a captcha is answered. Requires a valid ReCaptcha
# public/private key. Defaults to 'false'.
#
#enable_registration_captcha: true

# The API endpoint to use for verifying m.login.recaptcha responses.
# Defaults to "https://www.recaptcha.net/recaptcha/api/siteverify".
#
#recaptcha_siteverify_api: "https://my.recaptcha.site"


## TURN ##

# The public URIs of the TURN server to give to clients
#
#turn_uris: []

# The shared secret used to compute passwords for the TURN server
#
#turn_shared_secret: "YOUR_SHARED_SECRET"

# The Username and password if the TURN server needs them and
# does not use a token
#
#turn_username: "TURNSERVER_USERNAME"
#turn_password: "TURNSERVER_PASSWORD"

# How long generated TURN credentials last
#
#turn_user_lifetime: 1h

# Whether guests should be allowed to use the TURN server.
# This defaults to True, otherwise VoIP will be unreliable for guests.
# However, it does introduce a slight security risk as it allows users to
# connect to arbitrary endpoints without having first signed up for a
# valid account (e.g. by passing a CAPTCHA).
#
#turn_allow_guests: true


## Registration ##
#
# Registration can be rate-limited using the parameters in the "Ratelimiting"
# section of this file.

# Enable registration for new users.
#
#enable_registration: true

# Optional account validity configuration. This allows for accounts to be denied
# any request after a given period.
#
# Once this feature is enabled, Synapse will look for registered users without an
# expiration date at startup and will add one to every account it found using the
# current settings at that time.
# This means that, if a validity period is set, and Synapse is restarted (it will
# then derive an expiration date from the current validity period), and some time
# after that the validity period changes and Synapse is restarted, the users'
# expiration dates won't be updated unless their account is manually renewed. This
# date will be randomly selected within a range [now + period - d ; now + period],
# where d is equal to 10% of the validity period.
#
account_validity:
  # The account validity feature is disabled by default. Uncomment the
  # following line to enable it.
  #
  #enabled: true

  # The period after which an account is valid after its registration. When
  # renewing the account, its validity period will be extended by this amount
  # of time. This parameter is required when using the account validity
  # feature.
  #
  #period: 6w

  # The amount of time before an account's expiry date at which Synapse will
  # send an email to the account's email address with a renewal link. By
  # default, no such emails are sent.
  #
  # If you enable this setting, you will also need to fill out the 'email' and
  # 'public_baseurl' configuration sections.
  #
  #renew_at: 1w

  # The subject of the email sent out with the renewal link. '%(app)s' can be
  # used as a placeholder for the 'app_name' parameter from the 'email'
  # section.
  #
  # Note that the placeholder must be written '%(app)s', including the
  # trailing 's'.
  #
  # If this is not set, a default value is used.
  #
  #renew_email_subject: "Renew your %(app)s account"

  # Directory in which Synapse will try to find templates for the HTML files to
  # serve to the user when trying to renew an account. If not set, default
  # templates from within the Synapse package will be used.
  #
  #template_dir: "res/templates"

  # File within 'template_dir' giving the HTML to be displayed to the user after
  # they successfully renewed their account. If not set, default text is used.
  #
  #account_renewed_html_path: "account_renewed.html"

  # File within 'template_dir' giving the HTML to be displayed when the user
  # tries to renew an account with an invalid renewal token. If not set,
  # default text is used.
  #
  #invalid_token_html_path: "invalid_token.html"

  # Time that a user's session remains valid for, after they log in.
#
# Note that this is not currently compatible with guest logins.
#
# Note also that this is calculated at login time: changes are not applied
# retrospectively to users who have already logged in.
#
# By default, this is infinite.
#
#session_lifetime: 24h

# The user must provide all of the below types of 3PID when registering.
#
#registrations_require_3pid:
#  - email
#  - msisdn

# Explicitly disable asking for MSISDNs from the registration
# flow (overrides registrations_require_3pid if MSISDNs are set as required)
#
#disable_msisdn_registration: true

# Mandate that users are only allowed to associate certain formats of
# 3PIDs with accounts on this server.
#
#allowed_local_3pids:
#  - medium: email
#    pattern: '.*@matrix\.org'
#  - medium: email
#    pattern: '.*@vector\.im'
#  - medium: msisdn
#    pattern: '\+44'

# Enable 3PIDs lookup requests to identity servers from this server.
#
#enable_3pid_lookup: true

# If set, allows registration of standard or admin accounts by anyone who
# has the shared secret, even if registration is otherwise disabled.
#
registration_shared_secret: ":Cc*s^6_Xm*zcxu.jcxJXN=zGFaMzaUgmsP^gnCRFYg3,Tacsx"

# Set the number of bcrypt rounds used to generate password hash.
# Larger numbers increase the work factor needed to generate the hash.
# The default number is 12 (which equates to 2^12 rounds).
# N.B. that increasing this will exponentially increase the time required
# to register or login - e.g. 24 => 2^24 rounds which will take >20 mins.
#
#bcrypt_rounds: 12

# Allows users to register as guests without a password/email/etc, and
# participate in rooms hosted on this server which have been made
# accessible to anonymous users.
#
#allow_guest_access: false

# The identity server which we suggest that clients should use when users log
# in on this server.
#
# (By default, no suggestion is made, so it is left up to the client.
# This setting is ignored unless public_baseurl is also set.)
#
#default_identity_server: https://matrix.org

# Handle threepid (email/phone etc) registration and password resets through a set of
# *trusted* identity servers. Note that this allows the configured identity server to
# reset passwords for accounts!
#
# Be aware that if  'email ' is not set, and SMTP options have not been
# configured in the email config block, registration and user password resets via
# email will be globally disabled.
#
# Additionally, if  'msisdn ' is not set, registration and password resets via msisdn
# will be disabled regardless, and users will not be able to associate an msisdn
# identifier to their account. This is due to Synapse currently not supporting
# any method of sending SMS messages on its own.
#
# To enable using an identity server for operations regarding a particular third-party
# identifier type, set the value to the URL of that identity server as shown in the
# examples below.
#
# Servers handling the these requests must answer the  '/requestToken ' endpoints defined
# by the Matrix Identity Service API specification:
# https://matrix.org/docs/spec/identity_service/latest
#
# If a delegate is specified, the config option public_baseurl must also be filled out.
#
account_threepid_delegates:
    #email: https://example.com     # Delegate email sending to example.com
    #msisdn: http://localhost:8090  # Delegate SMS sending to this local process

    # Whether users are allowed to change their displayname after it has
# been initially set. Useful when provisioning users based on the
# contents of a third-party directory.
#
# Does not apply to server administrators. Defaults to 'true'
#
#enable_set_displayname: false

# Whether users are allowed to change their avatar after it has been
# initially set. Useful when provisioning users based on the contents
# of a third-party directory.
#
# Does not apply to server administrators. Defaults to 'true'
#
#enable_set_avatar_url: false

# Whether users can change the 3PIDs associated with their accounts
# (email address and msisdn).
#
# Defaults to 'true'
#
#enable_3pid_changes: false

# Users who register on this homeserver will automatically be joined
# to these rooms.
#
# By default, any room aliases included in this list will be created
# as a publicly joinable room when the first user registers for the
# homeserver. This behaviour can be customised with the settings below.
#
#auto_join_rooms:
#  - "#example:example.com"

# Where auto_join_rooms are specified, setting this flag ensures that the
# the rooms exist by creating them when the first user on the
# homeserver registers.
#
# By default the auto-created rooms are publicly joinable from any federated
# server. Use the autocreate_auto_join_rooms_federated and
# autocreate_auto_join_room_preset settings below to customise this behaviour.
#
# Setting to false means that if the rooms are not manually created,
# users cannot be auto-joined since they do not exist.
#
# Defaults to true. Uncomment the following line to disable automatically
# creating auto-join rooms.
#
#autocreate_auto_join_rooms: false

# Whether the auto_join_rooms that are auto-created are available via
# federation. Only has an effect if autocreate_auto_join_rooms is true.
#
# Note that whether a room is federated cannot be modified after
# creation.
#
# Defaults to true: the room will be joinable from other servers.
# Uncomment the following to prevent users from other homeservers from
# joining these rooms.
#
#autocreate_auto_join_rooms_federated: false

# The room preset to use when auto-creating one of auto_join_rooms. Only has an
# effect if autocreate_auto_join_rooms is true.
#
# This can be one of "public_chat", "private_chat", or "trusted_private_chat".
# If a value of "private_chat" or "trusted_private_chat" is used then
# auto_join_mxid_localpart must also be configured.
#
# Defaults to "public_chat", meaning that the room is joinable by anyone, including
# federated servers if autocreate_auto_join_rooms_federated is true (the default).
# Uncomment the following to require an invitation to join these rooms.
#
#autocreate_auto_join_room_preset: private_chat

# The local part of the user id which is used to create auto_join_rooms if
# autocreate_auto_join_rooms is true. If this is not provided then the
# initial user account that registers will be used to create the rooms.
#
# The user id is also used to invite new users to any auto-join rooms which
# are set to invite-only.
#
# It *must* be configured if autocreate_auto_join_room_preset is set to
# "private_chat" or "trusted_private_chat".
#
# Note that this must be specified in order for new users to be correctly
# invited to any auto-join rooms which have been set to invite-only (either
# at the time of creation or subsequently).
#
# Note that, if the room already exists, this user must be joined and
# have the appropriate permissions to invite new members.
#
#auto_join_mxid_localpart: system

# When auto_join_rooms is specified, setting this flag to false prevents
# guest accounts from being automatically joined to the rooms.
#
# Defaults to true.
#
#auto_join_rooms_for_guests: false


## Metrics ###

# Enable collection and rendering of performance metrics
#
#enable_metrics: false

# Enable sentry integration
# NOTE: While attempts are made to ensure that the logs don't contain
# any sensitive information, this cannot be guaranteed. By enabling
# this option the sentry server may therefore receive sensitive
# information, and it in turn may then diseminate sensitive information
# through insecure notification channels if so configured.
#
#sentry:
#    dsn: "..."

# Flags to enable Prometheus metrics which are not suitable to be
# enabled by default, either for performance reasons or limited use.
#
metrics_flags:
    # Publish synapse_federation_known_servers, a gauge of the number of
    # servers this homeserver knows about, including itself. May cause
    # performance problems on large homeservers.
    #
    #known_servers: true

    # Whether or not to report anonymized homeserver usage statistics.
#
report_stats: ` + utils.BoolToString(s.Spec.Homeserver.Values.ReportStats) + `

# The endpoint to report the anonymized homeserver usage statistics to.
# Defaults to https://matrix.org/report-usage-stats/push
#
#report_stats_endpoint: https://example.com/report-usage-stats/push


## API Configuration ##

# A list of event types that will be included in the room_invite_state
#
#room_invite_state_types:
#  - "m.room.join_rules"
#  - "m.room.canonical_alias"
#  - "m.room.avatar"
#  - "m.room.encryption"
#  - "m.room.name"


# A list of application service config files to use
#
#app_service_config_files:
#  - app_service_1.yaml
#  - app_service_2.yaml

# Uncomment to enable tracking of application service IP addresses. Implicitly
# enables MAU tracking for application service users.
#
#track_appservice_user_ips: true


# a secret which is used to sign access tokens. If none is specified,
# the registration_shared_secret is used, if one is given; otherwise,
# a secret key is derived from the signing key.
#
macaroon_secret_key: "EVr3uuImrTyxDVY1ukw*;r^zu1Y#8UkAp0@Bl8i9rzi~-+n95;"

# a secret which is used to calculate HMACs for form values, to stop
# falsification of values. Must be specified for the User Consent
# forms to work.
#
form_secret: "uD#~UE2pAzLUQIvj8x1;0iCzNL-UcUs1._WtUGXHRp@1Ogmyg4"

## Signing Keys ##

# Path to the signing key to sign messages with
#
signing_key_path: "data/` + s.Spec.Homeserver.Values.ServerName + `.signing.key"

# The keys that the server used to sign messages with but won't use
# to sign new messages.
#
old_signing_keys:
  # For each key,  'key ' should be the base64-encoded public key, and
  #  'expired_ts 'should be the time (in milliseconds since the unix epoch) that
  # it was last used.
  #
  # It is possible to build an entry from an old signing.key file using the
  #  'export_signing_key ' script which is provided with synapse.
  #
  # For example:
  #
  #"ed25519:id": { key: "base64string", expired_ts: 123456789123 }

  # How long key response published by this server is valid for.
# Used to set the valid_until_ts in /key/v2 APIs.
# Determines how quickly servers will query to check which keys
# are still valid.
#
#key_refresh_interval: 1d

# The trusted servers to download signing keys from.
#
# When we need to fetch a signing key, each server is tried in parallel.
#
# Normally, the connection to the key server is validated via TLS certificates.
# Additional security can be provided by configuring a  'verify key ', which
# will make synapse check that the response is signed by that key.
#
# This setting supercedes an older setting named  'perspectives '. The old format
# is still supported for backwards-compatibility, but it is deprecated.
#
# 'trusted_key_servers' defaults to matrix.org, but using it will generate a
# warning on start-up. To suppress this warning, set
# 'suppress_key_server_warning' to true.
#
# Options for each entry in the list include:
#
#    server_name: the name of the server. required.
#
#    verify_keys: an optional map from key id to base64-encoded public key.
#       If specified, we will check that the response is signed by at least
#       one of the given keys.
#
#    accept_keys_insecurely: a boolean. Normally, if  'verify_keys ' is unset,
#       and federation_verify_certificates is not  'true ', synapse will refuse
#       to start, because this would allow anyone who can spoof DNS responses
#       to masquerade as the trusted key server. If you know what you are doing
#       and are sure that your network environment provides a secure connection
#       to the key server, you can set this to  'true ' to override this
#       behaviour.
#
# An example configuration might look like:
#
#trusted_key_servers:
#  - server_name: "my_trusted_server.example.com"
#    verify_keys:
#      "ed25519:auto": "abcdefghijklmnopqrstuvwxyzabcdefghijklmopqr"
#  - server_name: "my_other_trusted_server.example.com"
#
trusted_key_servers:
  - server_name: "matrix.org"

  # Uncomment the following to disable the warning that is emitted when the
# trusted_key_servers include 'matrix.org'. See above.
#
#suppress_key_server_warning: true

# The signing keys to use when acting as a trusted key server. If not specified
# defaults to the server signing key.
#
# Can contain multiple keys, one per line.
#
#key_server_signing_keys_path: "key_server_signing_keys.key"


## Single sign-on integration ##

# The following settings can be used to make Synapse use a single sign-on
# provider for authentication, instead of its internal password database.
#
# You will probably also want to set the following options to  'false ' to
# disable the regular login/registration flows:
#   * enable_registration
#   * password_config.enabled
#
# You will also want to investigate the settings under the "sso" configuration
# section below.

# Enable SAML2 for registration and login. Uses pysaml2.
#
# At least one of  'sp_config ' or  'config_path ' must be set in this section to
# enable SAML login.
#
# Once SAML support is enabled, a metadata file will be exposed at
# https://<server>:<port>/_matrix/saml2/metadata.xml, which you may be able to
# use to configure your SAML IdP with. Alternatively, you can manually configure
# the IdP to use an ACS location of
# https://<server>:<port>/_matrix/saml2/authn_response.
#
saml2_config:
  #  'sp_config ' is the configuration for the pysaml2 Service Provider.
  # See pysaml2 docs for format of config.
  #
  # Default values will be used for the 'entityid' and 'service' settings,
  # so it is not normally necessary to specify them unless you need to
  # override them.
  #
  sp_config:
    # Point this to the IdP's metadata. You must provide either a local
    # file via the  'local ' attribute or (preferably) a URL via the
    #  'remote ' attribute.
    #
    #metadata:
    #  local: ["saml2/idp.xml"]
    #  remote:
    #    - url: https://our_idp/metadata.xml

    # Allowed clock difference in seconds between the homeserver and IdP.
    #
    # Uncomment the below to increase the accepted time difference from 0 to 3 seconds.
    #
    #accepted_time_diff: 3

    # By default, the user has to go to our login page first. If you'd like
    # to allow IdP-initiated login, set 'allow_unsolicited: true' in a
    # 'service.sp' section:
    #
    #service:
    #  sp:
    #    allow_unsolicited: true

    # The examples below are just used to generate our metadata xml, and you
    # may well not need them, depending on your setup. Alternatively you
    # may need a whole lot more detail - see the pysaml2 docs!

    #description: ["My awesome SP", "en"]
    #name: ["Test SP", "en"]

    #ui_info:
    #  display_name:
    #    - lang: en
    #      text: "Display Name is the descriptive name of your service."
    #  description:
    #    - lang: en
    #      text: "Description should be a short paragraph explaining the purpose of the service."
    #  information_url:
    #    - lang: en
    #      text: "https://example.com/terms-of-service"
    #  privacy_statement_url:
    #    - lang: en
    #      text: "https://example.com/privacy-policy"
    #  keywords:
    #    - lang: en
    #      text: ["Matrix", "Element"]
    #  logo:
    #    - lang: en
    #      text: "https://example.com/logo.svg"
    #      width: "200"
    #      height: "80"

    #organization:
    #  name: Example com
    #  display_name:
    #    - ["Example co", "en"]
    #  url: "http://example.com"

    #contact_person:
    #  - given_name: Bob
    #    sur_name: "the Sysadmin"
    #    email_address": ["admin@example.com"]
    #    contact_type": technical

  # Instead of putting the config inline as above, you can specify a
  # separate pysaml2 configuration file:
  #
  #config_path: "//sp_conf.py"

  # The lifetime of a SAML session. This defines how long a user has to
  # complete the authentication process, if allow_unsolicited is unset.
  # The default is 15 minutes.
  #
  #saml_session_lifetime: 5m

  # An external module can be provided here as a custom solution to
  # mapping attributes returned from a saml provider onto a matrix user.
  #
  user_mapping_provider:
    # The custom module's class. Uncomment to use a custom module.
    #
    #module: mapping_provider.SamlMappingProvider

    # Custom configuration values for the module. Below options are
    # intended for the built-in provider, they should be changed if
    # using a custom module. This section will be passed as a Python
    # dictionary to the module's  'parse_config ' method.
    #
    config:
      # The SAML attribute (after mapping via the attribute maps) to use
      # to derive the Matrix ID from. 'uid' by default.
      #
      # Note: This used to be configured by the
      # saml2_config.mxid_source_attribute option. If that is still
      # defined, its value will be used instead.
      #
      #mxid_source_attribute: displayName

      # The mapping system to use for mapping the saml attribute onto a
      # matrix ID.
      #
      # Options include:
      #  * 'hexencode' (which maps unpermitted characters to '=xx')
      #  * 'dotreplace' (which replaces unpermitted characters with
      #     '.').
      # The default is 'hexencode'.
      #
      # Note: This used to be configured by the
      # saml2_config.mxid_mapping option. If that is still defined, its
      # value will be used instead.
      #
      #mxid_mapping: dotreplace

  # In previous versions of synapse, the mapping from SAML attribute to
  # MXID was always calculated dynamically rather than stored in a
  # table. For backwards- compatibility, we will look for user_ids
  # matching such a pattern before creating a new account.
  #
  # This setting controls the SAML attribute which will be used for this
  # backwards-compatibility lookup. Typically it should be 'uid', but if
  # the attribute maps are changed, it may be necessary to change it.
  #
  # The default is 'uid'.
  #
  #grandfathered_mxid_source_attribute: upn

  # It is possible to configure Synapse to only allow logins if SAML attributes
  # match particular values. The requirements can be listed under
  #  'attribute_requirements ' as shown below. All of the listed attributes must
  # match for the login to be permitted.
  #
  #attribute_requirements:
  #  - attribute: userGroup
  #    value: "staff"
  #  - attribute: department
  #    value: "sales"

  # If the metadata XML contains multiple IdP entities then the  'idp_entityid '
  # option must be set to the entity to redirect users to.
  #
  # Most deployments only have a single IdP entity and so should omit this
  # option.
  #
  #idp_entityid: 'https://our_idp/entityid'


  # Enable OpenID Connect (OIDC) / OAuth 2.0 for registration and login.
#
# See https://github.com/matrix-org/synapse/blob/master/docs/openid.md
# for some example configurations.
#
oidc_config:
  # Uncomment the following to enable authorization against an OpenID Connect
  # server. Defaults to false.
  #
  #enabled: true

  # Uncomment the following to disable use of the OIDC discovery mechanism to
  # discover endpoints. Defaults to true.
  #
  #discover: false

  # the OIDC issuer. Used to validate tokens and (if discovery is enabled) to
  # discover the provider's endpoints.
  #
  # Required if 'enabled' is true.
  #
  #issuer: "https://accounts.example.com/"

  # oauth2 client id to use.
  #
  # Required if 'enabled' is true.
  #
  #client_id: "provided-by-your-issuer"

  # oauth2 client secret to use.
  #
  # Required if 'enabled' is true.
  #
  #client_secret: "provided-by-your-issuer"

  # auth method to use when exchanging the token.
  # Valid values are 'client_secret_basic' (default), 'client_secret_post' and
  # 'none'.
  #
  #client_auth_method: client_secret_post

  # list of scopes to request. This should normally include the "openid" scope.
  # Defaults to ["openid"].
  #
  #scopes: ["openid", "profile"]

  # the oauth2 authorization endpoint. Required if provider discovery is disabled.
  #
  #authorization_endpoint: "https://accounts.example.com/oauth2/auth"

  # the oauth2 token endpoint. Required if provider discovery is disabled.
  #
  #token_endpoint: "https://accounts.example.com/oauth2/token"

  # the OIDC userinfo endpoint. Required if discovery is disabled and the
  # "openid" scope is not requested.
  #
  #userinfo_endpoint: "https://accounts.example.com/userinfo"

  # URI where to fetch the JWKS. Required if discovery is disabled and the
  # "openid" scope is used.
  #
  #jwks_uri: "https://accounts.example.com/.well-known/jwks.json"

  # Uncomment to skip metadata verification. Defaults to false.
  #
  # Use this if you are connecting to a provider that is not OpenID Connect
  # compliant.
  # Avoid this in production.
  #
  #skip_verification: true

  # Whether to fetch the user profile from the userinfo endpoint. Valid
  # values are: "auto" or "userinfo_endpoint".
  #
  # Defaults to "auto", which fetches the userinfo endpoint if "openid" is included
  # in  'scopes '. Uncomment the following to always fetch the userinfo endpoint.
  #
  #user_profile_method: "userinfo_endpoint"

  # Uncomment to allow a user logging in via OIDC to match a pre-existing account instead
  # of failing. This could be used if switching from password logins to OIDC. Defaults to false.
  #
  #allow_existing_users: true

  # An external module can be provided here as a custom solution to mapping
  # attributes returned from a OIDC provider onto a matrix user.
  #
  user_mapping_provider:
    # The custom module's class. Uncomment to use a custom module.
    # Default is 'synapse.handlers.oidc_handler.JinjaOidcMappingProvider'.
    #
    # See https://github.com/matrix-org/synapse/blob/master/docs/sso_mapping_providers.md#openid-mapping-providers
    # for information on implementing a custom mapping provider.
    #
    #module: mapping_provider.OidcMappingProvider

    # Custom configuration values for the module. This section will be passed as
    # a Python dictionary to the user mapping provider module's  'parse_config '
    # method.
    #
    # The examples below are intended for the default provider: they should be
    # changed if using a custom provider.
    #
    config:
      # name of the claim containing a unique identifier for the user.
      # Defaults to  'sub ', which OpenID Connect compliant providers should provide.
      #
      #subject_claim: "sub"

      # Jinja2 template for the localpart of the MXID.
      #
      # When rendering, this template is given the following variables:
      #   * user: The claims returned by the UserInfo Endpoint and/or in the ID
      #     Token
      #
      # If this is not set, the user will be prompted to choose their
      # own username.
      #
      #localpart_template: "{{ user.preferred_username }}"

      # Jinja2 template for the display name to set on first login.
      #
      # If unset, no displayname will be set.
      #
      #display_name_template: "{{ user.given_name }} {{ user.last_name }}"

      # Jinja2 templates for extra attributes to send back to the client during
      # login.
      #
      # Note that these are non-standard and clients will ignore them without modifications.
      #
      #extra_attributes:
        #birthdate: "{{ user.birthdate }}"



        # Enable Central Authentication Service (CAS) for registration and login.
#
cas_config:
  # Uncomment the following to enable authorization against a CAS server.
  # Defaults to false.
  #
  #enabled: true

  # The URL of the CAS authorization endpoint.
  #
  #server_url: "https://cas-server.com"

  # The public URL of the homeserver.
  #
  #service_url: "https://homeserver.domain.com:8448"

  # The attribute of the CAS response to use as the display name.
  #
  # If unset, no displayname will be set.
  #
  #displayname_attribute: name

  # It is possible to configure Synapse to only allow logins if CAS attributes
  # match particular values. All of the keys in the mapping below must exist
  # and the values must match the given value. Alternately if the given value
  # is None then any value is allowed (the attribute just must exist).
  # All of the listed attributes must match for the login to be permitted.
  #
  #required_attributes:
  #  userGroup: "staff"
  #  department: None


  # Additional settings to use with single-sign on systems such as OpenID Connect,
# SAML2 and CAS.
#
sso:
    # A list of client URLs which are whitelisted so that the user does not
    # have to confirm giving access to their account to the URL. Any client
    # whose URL starts with an entry in the following list will not be subject
    # to an additional confirmation step after the SSO login is completed.
    #
    # WARNING: An entry such as "https://my.client" is insecure, because it
    # will also match "https://my.client.evil.site", exposing your users to
    # phishing attacks from evil.site. To avoid this, include a slash after the
    # hostname: "https://my.client/".
    #
    # If public_baseurl is set, then the login fallback page (used by clients
    # that don't natively support the required login flows) is whitelisted in
    # addition to any URLs in this list.
    #
    # By default, this list is empty.
    #
    #client_whitelist:
    #  - https://riot.im/develop
    #  - https://my.custom.client/

    # Directory in which Synapse will try to find the template files below.
    # If not set, or the files named below are not found within the template
    # directory, default templates from within the Synapse package will be used.
    #
    # Synapse will look for the following templates in this directory:
    #
    # * HTML page for a confirmation step before redirecting back to the client
    #   with the login token: 'sso_redirect_confirm.html'.
    #
    #   When rendering, this template is given three variables:
    #     * redirect_url: the URL the user is about to be redirected to. Needs
    #                     manual escaping (see
    #                     https://jinja.palletsprojects.com/en/2.11.x/templates/#html-escaping).
    #
    #     * display_url: the same as  'redirect_url', but with the query
    #                    parameters stripped. The intention is to have a
    #                    human-readable URL to show to users, not to use it as
    #                    the final address to redirect to. Needs manual escaping
    #                    (see https://jinja.palletsprojects.com/en/2.11.x/templates/#html-escaping).
    #
    #     * server_name: the homeserver's name.
    #
    # * HTML page which notifies the user that they are authenticating to confirm
    #   an operation on their account during the user interactive authentication
    #   process: 'sso_auth_confirm.html'.
    #
    #   When rendering, this template is given the following variables:
    #     * redirect_url: the URL the user is about to be redirected to. Needs
    #                     manual escaping (see
    #                     https://jinja.palletsprojects.com/en/2.11.x/templates/#html-escaping).
    #
    #     * description: the operation which the user is being asked to confirm
    #
    # * HTML page shown after a successful user interactive authentication session:
    #   'sso_auth_success.html'.
    #
    #   Note that this page must include the JavaScript which notifies of a successful authentication
    #   (see https://matrix.org/docs/spec/client_server/r0.6.0#fallback).
    #
    #   This template has no additional variables.
    #
    # * HTML page shown during single sign-on if a deactivated user (according to Synapse's database)
    #   attempts to login: 'sso_account_deactivated.html'.
    #
    #   This template has no additional variables.
    #
    # * HTML page to display to users if something goes wrong during the
    #   OpenID Connect authentication process: 'sso_error.html'.
    #
    #   When rendering, this template is given two variables:
    #     * error: the technical name of the error
    #     * error_description: a human-readable message for the error
    #
    # You can see the default templates at:
    # https://github.com/matrix-org/synapse/tree/master/synapse/res/templates
    #
    #template_dir: "res/templates"


    # JSON web token integration. The following settings can be used to make
# Synapse JSON web tokens for authentication, instead of its internal
# password database.
#
# Each JSON Web Token needs to contain a "sub" (subject) claim, which is
# used as the localpart of the mxid.
#
# Additionally, the expiration time ("exp"), not before time ("nbf"),
# and issued at ("iat") claims are validated if present.
#
# Note that this is a non-standard login type and client support is
# expected to be non-existent.
#
# See https://github.com/matrix-org/synapse/blob/master/docs/jwt.md.
#
#jwt_config:
    # Uncomment the following to enable authorization using JSON web
    # tokens. Defaults to false.
    #
    #enabled: true

    # This is either the private shared secret or the public key used to
    # decode the contents of the JSON web token.
    #
    # Required if 'enabled' is true.
    #
    #secret: "provided-by-your-issuer"

    # The algorithm used to sign the JSON web token.
    #
    # Supported algorithms are listed at
    # https://pyjwt.readthedocs.io/en/latest/algorithms.html
    #
    # Required if 'enabled' is true.
    #
    #algorithm: "provided-by-your-issuer"

    # The issuer to validate the "iss" claim against.
    #
    # Optional, if provided the "iss" claim will be required and
    # validated for all JSON web tokens.
    #
    #issuer: "provided-by-your-issuer"

    # A list of audiences to validate the "aud" claim against.
    #
    # Optional, if provided the "aud" claim will be required and
    # validated for all JSON web tokens.
    #
    # Note that if the "aud" claim is included in a JSON web token then
    # validation will fail without configuring audiences.
    #
    #audiences:
    #    - "provided-by-your-issuer"


    password_config:
   # Uncomment to disable password login
   #
   #enabled: false

   # Uncomment to disable authentication against the local password
   # database. This is ignored if  'enabled ' is false, and is only useful
   # if you have other password_providers.
   #
   #localdb_enabled: false

   # Uncomment and change to a secret random string for extra security.
   # DO NOT CHANGE THIS AFTER INITIAL SETUP!
   #
   #pepper: "EVEN_MORE_SECRET"

   # Define and enforce a password policy. Each parameter is optional.
   # This is an implementation of MSC2000.
   #
    policy:
      # Whether to enforce the password policy.
      # Defaults to 'false'.
      #
      #enabled: true

      # Minimum accepted length for a password.
      # Defaults to 0.
      #
      #minimum_length: 15

      # Whether a password must contain at least one digit.
      # Defaults to 'false'.
      #
      #require_digit: true

      # Whether a password must contain at least one symbol.
      # A symbol is any character that's not a number or a letter.
      # Defaults to 'false'.
      #
      #require_symbol: true

      # Whether a password must contain at least one lowercase letter.
      # Defaults to 'false'.
      #
      #require_lowercase: true

      # Whether a password must contain at least one lowercase letter.
      # Defaults to 'false'.
      #
      #require_uppercase: true

      ui_auth:
    # The number of milliseconds to allow a user-interactive authentication
    # session to be active.
    #
    # This defaults to 0, meaning the user is queried for their credentials
    # before every action, but this can be overridden to alow a single
    # validation to be re-used.  This weakens the protections afforded by
    # the user-interactive authentication process, by allowing for multiple
    # (and potentially different) operations to use the same validation session.
    #
    # Uncomment below to allow for credential validation to last for 15
    # seconds.
    #
    #session_timeout: 15000


    # Configuration for sending emails from Synapse.
#
email:
  # The hostname of the outgoing SMTP server to use. Defaults to 'localhost'.
  #
  #smtp_host: mail.server

  # The port on the mail server for outgoing SMTP. Defaults to 25.
  #
  #smtp_port: 587

  # Username/password for authentication to the SMTP server. By default, no
  # authentication is attempted.
  #
  #smtp_user: "exampleusername"
  #smtp_pass: "examplepassword"

  # Uncomment the following to require TLS transport security for SMTP.
  # By default, Synapse will connect over plain text, and will then switch to
  # TLS via STARTTLS *if the SMTP server supports it*. If this option is set,
  # Synapse will refuse to connect unless the server supports STARTTLS.
  #
  #require_transport_security: true

  # notif_from defines the "From" address to use when sending emails.
  # It must be set if email sending is enabled.
  #
  # The placeholder '%(app)s' will be replaced by the application name,
  # which is normally 'app_name' (below), but may be overridden by the
  # Matrix client application.
  #
  # Note that the placeholder must be written '%(app)s', including the
  # trailing 's'.
  #
  #notif_from: "Your Friendly %(app)s homeserver <noreply@example.com>"

  # app_name defines the default value for '%(app)s' in notif_from and email
  # subjects. It defaults to 'Matrix'.
  #
  #app_name: my_branded_matrix_server

  # Uncomment the following to enable sending emails for messages that the user
  # has missed. Disabled by default.
  #
  #enable_notifs: true

  # Uncomment the following to disable automatic subscription to email
  # notifications for new users. Enabled by default.
  #
  #notif_for_new_users: false

  # Custom URL for client links within the email notifications. By default
  # links will be based on "https://matrix.to".
  #
  # (This setting used to be called riot_base_url; the old name is still
  # supported for backwards-compatibility but is now deprecated.)
  #
  #client_base_url: "http://localhost/riot"

  # Configure the time that a validation email will expire after sending.
  # Defaults to 1h.
  #
  #validation_token_lifetime: 15m

  # The web client location to direct users to during an invite. This is passed
  # to the identity server as the org.matrix.web_client_location key. Defaults
  # to unset, giving no guidance to the identity server.
  #
  #invite_client_location: https://app.element.io

  # Directory in which Synapse will try to find the template files below.
  # If not set, or the files named below are not found within the template
  # directory, default templates from within the Synapse package will be used.
  #
  # Synapse will look for the following templates in this directory:
  #
  # * The contents of email notifications of missed events: 'notif_mail.html' and
  #   'notif_mail.txt'.
  #
  # * The contents of account expiry notice emails: 'notice_expiry.html' and
  #   'notice_expiry.txt'.
  #
  # * The contents of password reset emails sent by the homeserver:
  #   'password_reset.html' and 'password_reset.txt'
  #
  # * An HTML page that a user will see when they follow the link in the password
  #   reset email. The user will be asked to confirm the action before their
  #   password is reset: 'password_reset_confirmation.html'
  #
  # * HTML pages for success and failure that a user will see when they confirm
  #   the password reset flow using the page above: 'password_reset_success.html'
  #   and 'password_reset_failure.html'
  #
  # * The contents of address verification emails sent during registration:
  #   'registration.html' and 'registration.txt'
  #
  # * HTML pages for success and failure that a user will see when they follow
  #   the link in an address verification email sent during registration:
  #   'registration_success.html' and 'registration_failure.html'
  #
  # * The contents of address verification emails sent when an address is added
  #   to a Matrix account: 'add_threepid.html' and 'add_threepid.txt'
  #
  # * HTML pages for success and failure that a user will see when they follow
  #   the link in an address verification email sent when an address is added
  #   to a Matrix account: 'add_threepid_success.html' and
  #   'add_threepid_failure.html'
  #
  # You can see the default templates at:
  # https://github.com/matrix-org/synapse/tree/master/synapse/res/templates
  #
  #template_dir: "res/templates"

  # Subjects to use when sending emails from Synapse.
  #
  # The placeholder '%(app)s' will be replaced with the value of the 'app_name'
  # setting above, or by a value dictated by the Matrix client application.
  #
  # If a subject isn't overridden in this configuration file, the value used as
  # its example will be used.
  #
  #subjects:

    # Subjects for notification emails.
    #
    # On top of the '%(app)s' placeholder, these can use the following
    # placeholders:
    #
    #   * '%(person)s', which will be replaced by the display name of the user(s)
    #      that sent the message(s), e.g. "Alice and Bob".
    #   * '%(room)s', which will be replaced by the name of the room the
    #      message(s) have been sent to, e.g. "My super room".
    #
    # See the example provided for each setting to see which placeholder can be
    # used and how to use them.
    #
    # Subject to use to notify about one message from one or more user(s) in a
    # room which has a name.
    #message_from_person_in_room: "[%(app)s] You have a message on %(app)s from %(person)s in the %(room)s room..."
    #
    # Subject to use to notify about one message from one or more user(s) in a
    # room which doesn't have a name.
    #message_from_person: "[%(app)s] You have a message on %(app)s from %(person)s..."
    #
    # Subject to use to notify about multiple messages from one or more users in
    # a room which doesn't have a name.
    #messages_from_person: "[%(app)s] You have messages on %(app)s from %(person)s..."
    #
    # Subject to use to notify about multiple messages in a room which has a
    # name.
    #messages_in_room: "[%(app)s] You have messages on %(app)s in the %(room)s room..."
    #
    # Subject to use to notify about multiple messages in multiple rooms.
    #messages_in_room_and_others: "[%(app)s] You have messages on %(app)s in the %(room)s room and others..."
    #
    # Subject to use to notify about multiple messages from multiple persons in
    # multiple rooms. This is similar to the setting above except it's used when
    # the room in which the notification was triggered has no name.
    #messages_from_person_and_others: "[%(app)s] You have messages on %(app)s from %(person)s and others..."
    #
    # Subject to use to notify about an invite to a room which has a name.
    #invite_from_person_to_room: "[%(app)s] %(person)s has invited you to join the %(room)s room on %(app)s..."
    #
    # Subject to use to notify about an invite to a room which doesn't have a
    # name.
    #invite_from_person: "[%(app)s] %(person)s has invited you to chat on %(app)s..."

    # Subject for emails related to account administration.
    #
    # On top of the '%(app)s' placeholder, these one can use the
    # '%(server_name)s' placeholder, which will be replaced by the value of the
    # 'server_name' setting in your Synapse configuration.
    #
    # Subject to use when sending a password reset email.
    #password_reset: "[%(server_name)s] Password reset"
    #
    # Subject to use when sending a verification email to assert an address's
    # ownership.
    #email_validation: "[%(server_name)s] Validate your email"


    # Password providers allow homeserver administrators to integrate
# their Synapse installation with existing authentication methods
# ex. LDAP, external tokens, etc.
#
# For more information and known implementations, please see
# https://github.com/matrix-org/synapse/blob/master/docs/password_auth_providers.md
#
# Note: instances wishing to use SAML or CAS authentication should
# instead use the  'saml2_config ' or  'cas_config ' options,
# respectively.
#
password_providers:
#    # Example config for an LDAP auth provider
#    - module: "ldap_auth_provider.LdapAuthProvider"
#      config:
#        enabled: true
#        uri: "ldap://ldap.example.com:389"
#        start_tls: true
#        base: "ou=users,dc=example,dc=com"
#        attributes:
#           uid: "cn"
#           mail: "email"
#           name: "givenName"
#        #bind_dn:
#        #bind_password:
#        #filter: "(objectClass=posixAccount)"



## Push ##

push:
  # Clients requesting push notifications can either have the body of
  # the message sent in the notification poke along with other details
  # like the sender, or just the event ID and room ID ( 'event_id_only ').
  # If clients choose the former, this option controls whether the
  # notification request includes the content of the event (other details
  # like the sender are still included). For  'event_id_only ' push, it
  # has no effect.
  #
  # For modern android devices the notification content will still appear
  # because it is loaded by the app. iPhone, however will send a
  # notification saying only that a message arrived and who it came from.
  #
  # The default value is "true" to include message details. Uncomment to only
  # include the event ID and room ID in push notification payloads.
  #
  #include_content: false

  # When a push notification is received, an unread count is also sent.
  # This number can either be calculated as the number of unread messages
  # for the user, or the number of *rooms* the user has unread messages in.
  #
  # The default value is "true", meaning push clients will see the number of
  # rooms with unread messages in them. Uncomment to instead send the number
  # of unread messages.
  #
  #group_unread_count_by_room: false


  # Spam checkers are third-party modules that can block specific actions
# of local users, such as creating rooms and registering undesirable
# usernames, as well as remote users by redacting incoming events.
#
spam_checker:
   #- module: "my_custom_project.SuperSpamChecker"
   #  config:
   #    example_option: 'things'
   #- module: "some_other_project.BadEventStopper"
   #  config:
   #    example_stop_events_from: ['@bad:example.com']


   ## Rooms ##

# Controls whether locally-created rooms should be end-to-end encrypted by
# default.
#
# Possible options are "all", "invite", and "off". They are defined as:
#
# * "all": any locally-created room
# * "invite": any room created with the "private_chat" or "trusted_private_chat"
#             room creation presets
# * "off": this option will take no effect
#
# The default value is "off".
#
# Note that this option will only affect rooms created after it is set. It
# will also not affect rooms created by other servers.
#
#encryption_enabled_by_default_for_room_type: invite


# Uncomment to allow non-server-admin users to create groups on this server
#
#enable_group_creation: true

# If enabled, non server admins can only create groups with local parts
# starting with this prefix
#
#group_creation_prefix: "unofficial_"



# User Directory configuration
#
# 'enabled' defines whether users can search the user directory. If
# false then empty responses are returned to all queries. Defaults to
# true.
#
# 'search_all_users' defines whether to search all users visible to your HS
# when searching the user directory, rather than limiting to users visible
# in public rooms.  Defaults to false.  If you set it True, you'll have to
# rebuild the user_directory search indexes, see
# https://github.com/matrix-org/synapse/blob/master/docs/user_directory.md
#
#user_directory:
#  enabled: true
#  search_all_users: false


# User Consent configuration
#
# for detailed instructions, see
# https://github.com/matrix-org/synapse/blob/master/docs/consent_tracking.md
#
# Parts of this section are required if enabling the 'consent' resource under
# 'listeners', in particular 'template_dir' and 'version'.
#
# 'template_dir' gives the location of the templates for the HTML forms.
# This directory should contain one subdirectory per language (eg, 'en', 'fr'),
# and each language directory should contain the policy document (named as
# '<version>.html') and a success page (success.html).
#
# 'version' specifies the 'current' version of the policy document. It defines
# the version to be served by the consent resource if there is no 'v'
# parameter.
#
# 'server_notice_content', if enabled, will send a user a "Server Notice"
# asking them to consent to the privacy policy. The 'server_notices' section
# must also be configured for this to work. Notices will *not* be sent to
# guest users unless 'send_server_notice_to_guests' is set to true.
#
# 'block_events_error', if set, will block any attempts to send events
# until the user consents to the privacy policy. The value of the setting is
# used as the text of the error.
#
# 'require_at_registration', if enabled, will add a step to the registration
# process, similar to how captcha works. Users will be required to accept the
# policy before their account is created.
#
# 'policy_name' is the display name of the policy users will see when registering
# for an account. Has no effect unless  'require_at_registration ' is enabled.
# Defaults to "Privacy Policy".
#
#user_consent:
#  template_dir: res/templates/privacy
#  version: 1.0
#  server_notice_content:
#    msgtype: m.text
#    body: >-
#      To continue using this homeserver you must review and agree to the
#      terms and conditions at %(consent_uri)s
#  send_server_notice_to_guests: true
#  block_events_error: >-
#    To continue using this homeserver you must review and agree to the
#    terms and conditions at %(consent_uri)s
#  require_at_registration: false
#  policy_name: Privacy Policy
#



# Local statistics collection. Used in populating the room directory.
#
# 'bucket_size' controls how large each statistics timeslice is. It can
# be defined in a human readable short form -- e.g. "1d", "1y".
#
# 'retention' controls how long historical statistics will be kept for.
# It can be defined in a human readable short form -- e.g. "1d", "1y".
#
#
#stats:
#   enabled: true
#   bucket_size: 1d
#   retention: 1y


# Server Notices room configuration
#
# Uncomment this section to enable a room which can be used to send notices
# from the server to users. It is a special room which cannot be left; notices
# come from a special "notices" user id.
#
# If you uncomment this section, you *must* define the system_mxid_localpart
# setting, which defines the id of the user which will be used to send the
# notices.
#
# It's also possible to override the room name, the display name of the
# "notices" user, and the avatar for the user.
#
#server_notices:
#  system_mxid_localpart: notices
#  system_mxid_display_name: "Server Notices"
#  system_mxid_avatar_url: "mxc://server.com/oumMVlgDnLYFaPVkExemNVVZ"
#  room_name: "Server Notices"



# Uncomment to disable searching the public room list. When disabled
# blocks searching local and remote room lists for local and remote
# users by always returning an empty list for all queries.
#
#enable_room_list_search: false

# The  'alias_creation ' option controls who's allowed to create aliases
# on this server.
#
# The format of this option is a list of rules that contain globs that
# match against user_id, room_id and the new alias (fully qualified with
# server name). The action in the first rule that matches is taken,
# which can currently either be "allow" or "deny".
#
# Missing user_id/room_id/alias fields default to "*".
#
# If no rules match the request is denied. An empty list means no one
# can create aliases.
#
# Options for the rules include:
#
#   user_id: Matches against the creator of the alias
#   alias: Matches against the alias being created
#   room_id: Matches against the room ID the alias is being pointed at
#   action: Whether to "allow" or "deny" the request if the rule matches
#
# The default is:
#
#alias_creation_rules:
#  - user_id: "*"
#    alias: "*"
#    room_id: "*"
#    action: allow

# The  'room_list_publication_rules ' option controls who can publish and
# which rooms can be published in the public room list.
#
# The format of this option is the same as that for
#  'alias_creation_rules '.
#
# If the room has one or more aliases associated with it, only one of
# the aliases needs to match the alias rule. If there are no aliases
# then only rules with  'alias: * ' match.
#
# If no rules match the request is denied. An empty list means no one
# can publish rooms.
#
# Options for the rules include:
#
#   user_id: Matches against the creator of the alias
#   room_id: Matches against the room ID being published
#   alias: Matches against any current local or canonical aliases
#            associated with the room
#   action: Whether to "allow" or "deny" the request if the rule matches
#
# The default is:
#
#room_list_publication_rules:
#  - user_id: "*"
#    alias: "*"
#    room_id: "*"
#    action: allow


# Server admins can define a Python module that implements extra rules for
# allowing or denying incoming events. In order to work, this module needs to
# override the methods defined in synapse/events/third_party_rules.py.
#
# This feature is designed to be used in closed federations only, where each
# participating server enforces the same rules.
#
#third_party_event_rules:
#  module: "my_custom_project.SuperRulesSet"
#  config:
#    example_option: 'things'


## Opentracing ##

# These settings enable opentracing, which implements distributed tracing.
# This allows you to observe the causal chains of events across servers
# including requests, key lookups etc., across any server running
# synapse or any other other services which supports opentracing
# (specifically those implemented with Jaeger).
#
opentracing:
    # tracing is disabled by default. Uncomment the following line to enable it.
    #
    #enabled: true

    # The list of homeservers we wish to send and receive span contexts and span baggage.
    # See docs/opentracing.rst
    # This is a list of regexes which are matched against the server_name of the
    # homeserver.
    #
    # By default, it is empty, so no servers are matched.
    #
    #homeserver_whitelist:
    #  - ".*"

    # Jaeger can be configured to sample traces at different rates.
    # All configuration options provided by Jaeger can be set here.
    # Jaeger's configuration mostly related to trace sampling which
    # is documented here:
    # https://www.jaegertracing.io/docs/1.13/sampling/.
    #
    #jaeger_config:
    #  sampler:
    #    type: const
    #    param: 1

    #  Logging whether spans were started and reported
    #
    #  logging:
    #    false


    ## Workers ##

# Disables sending of outbound federation transactions on the main process.
# Uncomment if using a federation sender worker.
#
#send_federation: false

# It is possible to run multiple federation sender workers, in which case the
# work is balanced across them.
#
# This configuration must be shared between all federation sender workers, and if
# changed all federation sender workers must be stopped at the same time and then
# started, to ensure that all instances are running with the same config (otherwise
# events may be dropped).
#
#federation_sender_instances:
#  - federation_sender1

# When using workers this should be a map from  'worker_name ' to the
# HTTP replication listener of the worker, if configured.
#
#instance_map:
#  worker1:
#    host: localhost
#    port: 8034

# Experimental: When using workers you can define which workers should
# handle event persistence and typing notifications. Any worker
# specified here must also be in the  'instance_map '.
#
#stream_writers:
#  events: worker1
#  typing: worker1

# The worker that is used to run background tasks (e.g. cleaning up expired
# data). If not provided this defaults to the main process.
#
#run_background_tasks_on: worker1

# A shared secret used by the replication APIs to authenticate HTTP requests
# from workers.
#
# By default this is unused and traffic is not authenticated.
#
#worker_replication_secret: "


# Configuration for Redis when using workers. This *must* be enabled when
# using workers (unless using old style direct TCP configuration).
#
redis:
  # Uncomment the below to enable Redis support.
  #
  #enabled: true

  # Optional host and port to use to connect to redis. Defaults to
  # localhost and 6379
  #
  #host: localhost
  #port: 6379

  # Optional password if configured on the Redis instance
  #
  #password: <secret_password>


  # vim:ft=yaml
  `

	cm := &corev1.ConfigMap{
		ObjectMeta: objectMeta,
		Data:       map[string]string{"homeserver.yaml": homeserverYaml},
	}

	// Set Synapse instance as the owner and controller
	if err := ctrl.SetControllerReference(s, cm, r.Scheme); err != nil {
		return &corev1.ConfigMap{}, err
	}

	return cm, nil
}

// copyInputSynapseConfigMap is a function of type FnWithRequest, to be
// called in the main reconciliation loop.
//
// It creates a copy of the user-provided ConfigMap for synapse, defined in
// synapse.Spec.Homeserver.ConfigMap
func (r *SynapseReconciler) copyInputSynapseConfigMap(ctx context.Context, req ctrl.Request) (*ctrl.Result, error) {
	s := &synapsev1alpha1.Synapse{}
	if r, err := r.getLatestSynapse(ctx, req, s); subreconciler.ShouldHaltOrRequeue(r, err) {
		return r, err
	}

	objectMetaForSynapse := reconcile.SetObjectMeta(s.Name, s.Namespace, map[string]string{})

	desiredConfigMap, err := r.configMapForSynapseCopy(s, objectMetaForSynapse)
	if err != nil {
		return subreconciler.RequeueWithError(err)
	}

	// Create a copy of the inputConfigMap defined in Spec.Homeserver.ConfigMap
	// Here we use the configMapForSynapseCopy function as createResourceFunc
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

// The ConfigMap returned by configMapForSynapseCopy is a copy of the ConfigMap
// defined in Spec.Homeserver.ConfigMap.
func (r *SynapseReconciler) configMapForSynapseCopy(
	s *synapsev1alpha1.Synapse,
	objectMeta metav1.ObjectMeta,
) (*corev1.ConfigMap, error) {
	var copyConfigMap *corev1.ConfigMap

	sourceConfigMapName := s.Spec.Homeserver.ConfigMap.Name
	sourceConfigMapNamespace := utils.ComputeNamespace(s.Namespace, s.Spec.Homeserver.ConfigMap.Namespace)

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
	if err := ctrl.SetControllerReference(s, copyConfigMap, r.Scheme); err != nil {
		return nil, err
	}

	return copyConfigMap, nil
}

// parseInputSynapseConfigMap is a function of type FnWithRequest, to be
// called in the main reconciliation loop.
//
// It checks that the ConfigMap referenced by
// synapse.Spec.Homeserver.ConfigMap.Name exists and extrats the server_name
// and report_stats values.
func (r *SynapseReconciler) parseInputSynapseConfigMap(ctx context.Context, req ctrl.Request) (*ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)

	s := &synapsev1alpha1.Synapse{}
	if r, err := r.getLatestSynapse(ctx, req, s); subreconciler.ShouldHaltOrRequeue(r, err) {
		return r, err
	}

	var inputConfigMap corev1.ConfigMap // the user-provided ConfigMap. It should contain a valid homeserver.yaml
	ConfigMapName := s.Spec.Homeserver.ConfigMap.Name
	ConfigMapNamespace := utils.ComputeNamespace(s.Namespace, s.Spec.Homeserver.ConfigMap.Namespace)
	keyForInputConfigMap := types.NamespacedName{
		Name:      ConfigMapName,
		Namespace: ConfigMapNamespace,
	}

	// Get and validate the inputConfigMap
	if err := r.Get(ctx, keyForInputConfigMap, &inputConfigMap); err != nil {
		reason := "ConfigMap " + ConfigMapName + " does not exist in namespace " + ConfigMapNamespace
		if err := r.setFailedState(ctx, s, reason); err != nil {
			log.Error(err, "Error updating Synapse State")
		}

		log.Error(
			err,
			"Failed to get ConfigMap",
			"ConfigMap.Namespace",
			ConfigMapNamespace,
			"ConfigMap.Name",
			ConfigMapName,
		)
		return subreconciler.RequeueWithDelayAndError(time.Duration(30), err)
	}

	if err := r.ParseHomeserverConfigMap(ctx, s, inputConfigMap); err != nil {
		return subreconciler.RequeueWithDelayAndError(time.Duration(30), err)
	}

	err, has_patched := r.updateSynapseStatus(ctx, s)
	if err != nil {
		log.Error(err, "Error updating Synapse Status")
		return subreconciler.RequeueWithError(err)
	}
	if has_patched {
		return subreconciler.Requeue()
	}

	return subreconciler.ContinueReconciling()
}

// ParseHomeserverConfigMap loads the ConfigMap, which name is determined by
// Spec.Homeserver.ConfigMap.Name, run validation checks and fetch necesarry
// value needed to configure the Synapse Deployment.
func (r *SynapseReconciler) ParseHomeserverConfigMap(ctx context.Context, synapse *synapsev1alpha1.Synapse, cm corev1.ConfigMap) error {
	log := ctrllog.FromContext(ctx)

	// TODO:
	// - Ensure that key path is and log config file path are in /data
	// - Otherwise, edit homeserver.yaml with new paths

	// Load and validate homeserver.yaml
	homeserver, err := utils.LoadYAMLFileFromConfigMapData(cm, "homeserver.yaml")
	if err != nil {
		return err
	}

	// Fetch server_name and report_stats
	if _, ok := homeserver["server_name"]; !ok {
		err := errors.New("missing server_name key in homeserver.yaml")
		log.Error(err, "Missing server_name key in homeserver.yaml")
		return err
	}
	server_name, ok := homeserver["server_name"].(string)
	if !ok {
		err := errors.New("error converting server_name to string")
		log.Error(err, "Error converting server_name to string")
		return err
	}

	if _, ok := homeserver["report_stats"]; !ok {
		err := errors.New("missing report_stats key in homeserver.yaml")
		log.Error(err, "Missing report_stats key in homeserver.yaml")
		return err
	}
	report_stats, ok := homeserver["report_stats"].(bool)
	if !ok {
		err := errors.New("error converting report_stats to bool")
		log.Error(err, "Error converting report_stats to bool")
		return err
	}

	// Populate the Status.HomeserverConfiguration with values defined in homeserver.yaml
	synapse.Status.HomeserverConfiguration.ServerName = server_name
	synapse.Status.HomeserverConfiguration.ReportStats = report_stats

	log.Info(
		"Loaded homeserver.yaml from ConfigMap successfully",
		"server_name:", synapse.Status.HomeserverConfiguration.ServerName,
		"report_stats:", synapse.Status.HomeserverConfiguration.ReportStats,
	)

	return nil
}

// updateSynapseConfigMapForPostgresCluster is a function of type
// FnWithRequest, to be called in the main reconciliation loop.
//
// It configures the 'database' section of homeserver.yaml to allow Synapse to
// connect to the newly created PostgresCluster instance.
func (r *SynapseReconciler) updateSynapseConfigMapForPostgresCluster(ctx context.Context, req ctrl.Request) (*ctrl.Result, error) {
	s := &synapsev1alpha1.Synapse{}
	if r, err := r.getLatestSynapse(ctx, req, s); subreconciler.ShouldHaltOrRequeue(r, err) {
		return r, err
	}

	keyForSynapse := types.NamespacedName{
		Name:      s.Name,
		Namespace: s.Namespace,
	}

	if err := utils.UpdateConfigMap(
		ctx,
		r.Client,
		keyForSynapse,
		s,
		r.updateHomeserverWithPostgreSQLInfos,
		"homeserver.yaml",
	); err != nil {
		return subreconciler.RequeueWithError(err)
	}

	return subreconciler.ContinueReconciling()
}

func (r *SynapseReconciler) updateHomeserverWithPostgreSQLInfos(
	obj client.Object,
	homeserver map[string]interface{},
) error {
	s := obj.(*synapsev1alpha1.Synapse)

	databaseData, err := r.fetchDatabaseDataFromSynapseStatus(*s)
	if err != nil {
		return err
	}

	// Save new database section of homeserver.yaml
	homeserver["database"] = databaseData
	return nil
}

func (r *SynapseReconciler) fetchDatabaseDataFromSynapseStatus(s synapsev1alpha1.Synapse) (map[string]interface{}, error) {
	databaseData := HomeserverPgsqlDatabase{}

	// Check if s.Status.DatabaseConnectionInfo contains necessary information
	if s.Status.DatabaseConnectionInfo == (synapsev1alpha1.SynapseStatusDatabaseConnectionInfo{}) {
		err := errors.New("missing DatabaseConnectionInfo in Synapse status")
		return map[string]interface{}{}, err
	}

	if s.Status.DatabaseConnectionInfo.User == "" {
		err := errors.New("missing User in DatabaseConnectionInfo")
		return map[string]interface{}{}, err
	}

	if s.Status.DatabaseConnectionInfo.Password == "" {
		err := errors.New("missing Password in DatabaseConnectionInfo")
		return map[string]interface{}{}, err
	}
	decodedPassword := base64decode([]byte(s.Status.DatabaseConnectionInfo.Password))

	if s.Status.DatabaseConnectionInfo.DatabaseName == "" {
		err := errors.New("missing DatabaseName in DatabaseConnectionInfo")
		return map[string]interface{}{}, err
	}

	if s.Status.DatabaseConnectionInfo.ConnectionURL == "" {
		err := errors.New("missing ConnectionURL in DatabaseConnectionInfo")
		return map[string]interface{}{}, err
	}
	connectionURL := strings.Split(s.Status.DatabaseConnectionInfo.ConnectionURL, ":")
	if len(connectionURL) < 2 {
		err := errors.New("error parsing the Connection URL with value: " + s.Status.DatabaseConnectionInfo.ConnectionURL)
		return map[string]interface{}{}, err
	}
	port, err := strconv.ParseInt(connectionURL[1], 10, 64)
	if err != nil {
		return map[string]interface{}{}, err
	}

	// Populate databaseData
	databaseData.Name = "psycopg2"
	databaseData.Args.User = s.Status.DatabaseConnectionInfo.User
	databaseData.Args.Password = decodedPassword
	databaseData.Args.Database = s.Status.DatabaseConnectionInfo.DatabaseName
	databaseData.Args.Host = connectionURL[0]
	databaseData.Args.Port = port
	databaseData.Args.CpMin = 5
	databaseData.Args.CpMax = 10

	// Convert databaseData into a map[string]interface{}
	databaseDataMap, err := utils.ConvertStructToMap(databaseData)
	if err != nil {
		return map[string]interface{}{}, err
	}

	return databaseDataMap, nil
}

// updateSynapseConfigMapForHeisenbridge is a function of type
// FnWithRequest, to be called in the main reconciliation loop.
//
// It registers the heisenbridge as an application service in the
// homeserver.yaml config file.
func (r *SynapseReconciler) updateSynapseConfigMapForHeisenbridge(ctx context.Context, req ctrl.Request) (*ctrl.Result, error) {
	s := &synapsev1alpha1.Synapse{}
	if r, err := r.getLatestSynapse(ctx, req, s); subreconciler.ShouldHaltOrRequeue(r, err) {
		return r, err
	}

	keyForSynapse := types.NamespacedName{
		Name:      s.Name,
		Namespace: s.Namespace,
	}

	// Update the Synapse ConfigMap to enable heisenbridge
	if err := utils.UpdateConfigMap(
		ctx,
		r.Client,
		keyForSynapse,
		s,
		r.updateHomeserverWithHeisenbridgeInfos,
		"homeserver.yaml",
	); err != nil {
		return subreconciler.RequeueWithError(err)
	}

	return subreconciler.ContinueReconciling()
}

// updateHomeserverWithHeisenbridgeInfos is a function of type updateDataFunc
// function to be passed as an argument in a call to utils.UpdateConfigMap.
//
// It enables the Heisenbridge as an AppService in Synapse.
func (r *SynapseReconciler) updateHomeserverWithHeisenbridgeInfos(
	_ client.Object,
	homeserver map[string]interface{},
) error {
	// Add heisenbridge configuration file to the list of application services
	r.addAppServiceToHomeserver(homeserver, "/data-heisenbridge/heisenbridge.yaml")
	return nil
}

// updateSynapseConfigMapForMautrixSignal is a function of type
// FnWithRequest, to be called in the main reconciliation loop.
//
// It registers the mautrix-signal bridge as an application service in the
// homeserver.yaml config file.
func (r *SynapseReconciler) updateSynapseConfigMapForMautrixSignal(ctx context.Context, req ctrl.Request) (*ctrl.Result, error) {
	s := &synapsev1alpha1.Synapse{}
	if r, err := r.getLatestSynapse(ctx, req, s); subreconciler.ShouldHaltOrRequeue(r, err) {
		return r, err
	}

	keyForSynapse := types.NamespacedName{
		Name:      s.Name,
		Namespace: s.Namespace,
	}

	// Update the Synapse ConfigMap to enable mautrix-signal
	if err := utils.UpdateConfigMap(
		ctx,
		r.Client,
		keyForSynapse,
		s,
		r.updateHomeserverWithMautrixSignalInfos,
		"homeserver.yaml",
	); err != nil {
		return subreconciler.RequeueWithError(err)
	}

	return subreconciler.ContinueReconciling()
}

// updateHomeserverWithMautrixSignalInfos is a function of type updateDataFunc
// function to be passed as an argument in a call to utils.UpdateConfigMap.
//
// It enables the mautrix-signal bridge as an AppService in Synapse.
func (r *SynapseReconciler) updateHomeserverWithMautrixSignalInfos(
	_ client.Object,
	homeserver map[string]interface{},
) error {
	// Add mautrix-signal configuration file to the list of application services
	r.addAppServiceToHomeserver(homeserver, "/data-mautrixsignal/registration.yaml")
	return nil
}

func (r *SynapseReconciler) addAppServiceToHomeserver(
	homeserver map[string]interface{},
	configFilePath string,
) {
	homeserverAppService, ok := homeserver["app_service_config_files"].([]string)
	if !ok {
		// "app_service_config_files" key not present, or malformed. Overwrite with
		// the given app_service config file.
		homeserver["app_service_config_files"] = []string{configFilePath}
	} else {
		// There are already app services registered. Adding to the list.
		homeserver["app_service_config_files"] = append(homeserverAppService, configFilePath)
	}
}
