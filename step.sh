#!/bin/bash

THIS_SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

set -e

#
# Input validation
if [ -z "${apk_path}" ] ; then
	echo "[!] Missing required input: apk_path"
	exit 1
fi

if [ -z "${keystore_url}" ] ; then
	echo "[!] Missing required input: keystore_url"
	exit 1
fi

if [ -z "${keystore_password}" ] ; then
	echo "[!] Missing required input: keystore_password"
	exit 1
fi

if [ -z "${keystore_alias}" ] ; then
	echo "[!] Missing required input: keystore_alias"
	exit 1
fi

if [ -z "${private_key_password}" ] ; then
	echo "[!] Missing required input: private_key_password"
	exit 1
fi

echo
echo "(i) Required inputs are provided"


#
# Download keystore
keystore_path=""
file_prefix="file://"
if [[ "${keystore_url}" == ${file_prefix}* ]] ; then
	keystore_path=${keystore_url#$file_prefix}

	echo
	echo "==> Using local keystore: ${keystore_path}"
else
	echo
	echo "==> Downloading keystore: ${keystore_url}"

	tmp_dir=$(mktemp -d)
	keystore_path="${tmp_dir}/keystore.jks"
	curl -fL "${keystore_url}" > "${keystore_path}"
fi


#
# Sign apk
echo
echo "==> Signing apk"
file_name=$(basename "${apk_path}")
dir_name=$(dirname "${apk_path}")
tmp_signed_apk_path="${dir_name}/tmp_signed-${file_name}"

jarsigner="/usr/bin/jarsigner"

"${jarsigner}" -verbose \
  -sigalg SHA1withRSA \
  -digestalg SHA1 \
  -keystore "${keystore_path}" \
  -storepass "${keystore_password}" \
  -signedjar "${tmp_signed_apk_path}" "${apk_path}" "${keystore_alias}"


#
# Now zipalign it.
# The -v parameter tells zipalign to verify the APK afterwards.
echo
echo "==> Ziping apk"
zipalign=$(ruby "$THIS_SCRIPT_DIR/export_latest_zipalign.rb")
if [ $? -ne 0 ] ; then
	echo
	echo " (!) Failed to get ziplaign path"
	echo " (!) error: $zipalign"
	exit 1
fi
echo "zipalign: $zipalign"

signed_apk_path="${dir_name}/signed-${file_name}"

${zipalign} -f -v 4 ${tmp_signed_apk_path} ${signed_apk_path}


#
# Verifying
echo
echo "==> Verifying the ${signed_apk_path}"
out=$(${jarsigner} -verify -verbose -certs ${signed_apk_path})
if [[ "$out" == *"jar verified."* ]] ; then
	echo " (i) apk successfully signed"
else
	echo "verification out:"
	echo $out
	echo " (!) apk isn't signed"
	exit 1
fi


#
# Exporting signed ipa
echo
echo " (i) The signed apk is now available at: ${signed_apk_path}"
envman add --key BITRISE_SIGNED_APK_PATH --value "${signed_apk_path}"


#
# Cleanup
rm ${tmp_signed_apk_path}
