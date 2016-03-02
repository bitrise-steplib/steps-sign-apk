#!/bin/bash

THIS_SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

set -e

#
# Input validation
if [ -z "${apk_path}" ] ; then
	printf "\e[31mError: Missing required input: apk_path\e[0m\n"
	exit 1
fi

if [ -z "${keystore_url}" ] ; then
	printf "\e[31mError: Missing required input: keystore_url\e[0m\n"
	exit 1
fi

if [ -z "${keystore_password}" ] ; then
	printf "\e[31mError: Missing required input: keystore_password\e[0m\n"
	exit 1
fi

if [ -z "${keystore_alias}" ] ; then
	printf "\e[31mError: Missing required input: keystore_alias\e[0m\n"
	exit 1
fi

if [ -z "${private_key_password}" ] ; then
	printf "\e[31mError: Missing required input: private_key_password\e[0m\n"
	exit 1
fi

#
# Download keystore
keystore_path=""
file_prefix="file://"
if [[ "${keystore_url}" == ${file_prefix}* ]] ; then
	keystore_path=${keystore_url#$file_prefix}
else
	echo
	printf "\e[34mDownloading keystore\e[0m\n"

	tmp_dir=$(mktemp -d)
	keystore_path="${tmp_dir}/keystore.jks"
	curl -fL "${keystore_url}" > "${keystore_path}"
fi


#
# Sign apk
echo
printf "\e[34mSigning apk\e[0m\n"
echo "  Using keystore: ${keystore_path}"

file_name=$(basename "${apk_path}")
dir_name=$(dirname "${apk_path}")
tmp_signed_apk_path="${dir_name}/tmp_signed-${file_name}"

jarsigner="/usr/bin/jarsigner"
"${jarsigner}" \
  -keystore "${keystore_path}" \
  -storepass "${keystore_password}" \
	${jarsigner_options} \
  -signedjar "${tmp_signed_apk_path}" "${apk_path}" "${keystore_alias}"

#
# Now zipalign it.
# The -v parameter tells zipalign to verify the APK afterwards.
echo "  Aligning the APK"
zipalign=$(ruby "$THIS_SCRIPT_DIR/export_latest_zipalign.rb")
if [ $? -ne 0 ] ; then
	printf "\e[31mError: ${zipalign}\e[0m\n"
	exit 1
fi

signed_apk_path="${dir_name}/signed-${file_name}"
${zipalign} -f 4 ${tmp_signed_apk_path} ${signed_apk_path}

#
# Verifying
echo "  Verifying the signed APK"
out=$(${jarsigner} -verify -verbose -certs ${signed_apk_path})
if [[ "$out" != *"jar verified."* ]] ; then
	printf "\e[31mError: Failed to sign APK\e[0m\n"
	exit 1
fi

#
# Exporting signed ipa
printf "  \e[32mSigned APK created at path: ${signed_apk_path}\e[0m\n"
envman add --key BITRISE_SIGNED_APK_PATH --value "${signed_apk_path}"

#
# Cleanup
rm ${tmp_signed_apk_path}
