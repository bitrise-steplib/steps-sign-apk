#!/bin/bash

THIS_SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

# -----------------------
# --- Functions
# -----------------------

RESTORE='\033[0m'
RED='\033[00;31m'
YELLOW='\033[00;33m'
BLUE='\033[00;34m'
GREEN='\033[00;32m'

function color_echo {
	color=$1
	msg=$2
	echo -e "${color}${msg}${RESTORE}"
}

function echo_fail {
	msg=$1
	echo
	color_echo "${RED}" "${msg}"
	exit 1
}

function echo_warn {
	msg=$1
	color_echo "${YELLOW}" "${msg}"
}

function echo_info {
	msg=$1
	echo
	color_echo "${BLUE}" "${msg}"
}

function echo_details {
	msg=$1
	echo "  ${msg}"
}

function echo_done {
	msg=$1
	color_echo "${GREEN}" "  ${msg}"
}

function validate_required_input {
	key=$1
	value=$2
	if [ -z "${value}" ] ; then
		echo_fail "[!] Missing required input: ${key}"
	fi
}

# -----------------------
# --- Main
# -----------------------

# Input validation
echo_info "Configs:"
echo_details "* apk_path: ${apk_path}"
echo_details "* keystore_url: ${keystore_url}"
echo_details "* keystore_password: ***"
echo_details "* keystore_alias: ***"
echo_details "* private_key_password: ***"

validate_required_input "apk_path" "${apk_path}"
validate_required_input "keystore_url" "${keystore_url}"
validate_required_input "keystore_password" "${keystore_password}"
validate_required_input "keystore_alias" "${keystore_alias}"
validate_required_input "private_key_password" "${private_key_password}"

# Download keystore
echo_info "Preparing keystore..."

keystore_path=""
file_prefix="file://"
if [[ "${keystore_url}" == ${file_prefix}* ]] ; then
	echo_details "keystore file path provided"
	keystore_path=${keystore_url#$file_prefix}
else
	echo_details "downloading remote keystore"

	tmp_dir=$(mktemp -d)
	keystore_path="${tmp_dir}/keystore.jks"
	curl -fL "${keystore_url}" > "${keystore_path}"
fi

# Sign apk
echo_info "Signing APK..."
echo_details "using keystore: ${keystore_path}"

file_name=$(basename "${apk_path}")
dir_name=$(dirname "${apk_path}")
tmp_signed_apk_path="${dir_name}/tmp_signed-${file_name}"

jarsigner="/usr/bin/jarsigner"
"${jarsigner}" \
	-keystore "${keystore_path}" \
	-storepass "${keystore_password}" \
	-keypass "${private_key_password}" \
	${jarsigner_options} \
	-signedjar "${tmp_signed_apk_path}" "${apk_path}" "${keystore_alias}"

if [ $? -ne 0 ] ; then
	echo_fail "Failed to sign APK"
fi

# Now zipalign it.
# The -v parameter tells zipalign to verify the APK afterwards.
echo_info "Aligning the APK..."

zipalign=$(ruby "$THIS_SCRIPT_DIR/export_latest_zipalign.rb")

if [ $? -ne 0 ] ; then
	echo_warn "${zipalign}"
	echo_fail "Failed to aligning the APK"
fi

signed_apk_path="${dir_name}/signed-${file_name}"
${zipalign} -f 4 ${tmp_signed_apk_path} ${signed_apk_path}

# Verifying
echo_info "Verifying the signed APK..."

out=$(${jarsigner} -verify -verbose -certs ${signed_apk_path})
if [[ $out =~ .*"jar verified".* ]] ; then
	echo_details "APK verified"
else
	echo_warn "${out}"
	echo_fail "Failed to verify APK"
fi

# Exporting signed ipa
envman add --key BITRISE_SIGNED_APK_PATH --value "${signed_apk_path}"

echo
echo_done "Signed APK created at path: ${signed_apk_path}"
echo_details "Exported BITRISE_SIGNED_APK_PATH"

#
# Cleanup
rm ${tmp_signed_apk_path}
