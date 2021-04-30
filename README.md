# Android Sign ![Bitrise Build Status](https://app.bitrise.io/app/3b968e65d584db2a.svg?token=Yk1LUEjLZtIjeIW4OOZvKw&branch=master) [![Bitrise Step Version](https://shields.io/github/v/release/bitrise-steplib/steps-sign-apk?include_prereleases)](https://github.com/bitrise-steplib/steps-sign-apk/releases) ![GitHub License](https://img.shields.io/badge/license-MIT-lightgrey.svg) [![Bitrise Community](https://img.shields.io/badge/community-Bitrise%20Discuss-lightgrey)](https://discuss.bitrise.io/)

Signs your APK or Android App Bundle before uploading it to Google Play Store.

Once you have uploaded your keystore file and provided your keystore credentials on the **Code Signing** tab of the Workflow Editor, the **Android Sign** Step signs your APK digitally. 
Bitrise assigns Environment Variables to the uploaded file and credentials, and uses those in the respective fields of the **Android Sign** Step. 
Once the Step runs, it produces a signed APK or App Bundle which will be used as the input value of the **App file path** field in the **Google Play Deploy** Step.    blabla

### Configuring the Step

1. Add the **Android Sign** Step after a build Step in your deploy workflow.
2. Upload the keystore file to the **Upload file** field on the **Code Signing** tab.
3. Provide your keystore password, keystore alias and private key password to the relevant fields on the **Code Signing** tab.
4. Run your build.


### Troubleshooting
Make sure you have the **Android Sign** Step right after a build Steps but before **Deploy to Google Play** Step in your deploy workflow.
If you wish to get your Android project signed automatically, use the **Android Sign** Step and do not set any gradle task for the signing, otherwise, the Step will fail.


### Useful links
- [Android code signing using Android Sign Step](https://devcenter.bitrise.io/code-signing/android-code-signing/android-code-signing-using-bitrise-sign-apk-step/)
- [Android deployment](https://devcenter.bitrise.io/deploy/android-deploy/android-deployment-index/)


### Related Steps
- [Android Build](https://www.bitrise.io/integrations/steps/android-build)
- [Gradle Runner](https://www.bitrise.io/integrations/steps/gradle-runner)
- [Deploy to Bitrise.io](https://www.bitrise.io/integrations/steps/deploy-to-bitrise-io)

## Examples

### Building a bundle and signing it

1. Build an Android App Bundle:
```
---
format_version: '8'
default_step_lib_source: https://github.com/bitrise-io/bitrise-steplib.git
project_type: android
workflows:
  release:
    envs:
    - PROJECT_LOCATION: .
    - MODULE:
    - VARIANT: release
    # If the Android keystore is configured in the workflow editor, BITRISEIO_ANDROID_KEYSTORE* envs will be set automatically
    - BITRISEIO_ANDROID_KEYSTORE_URL: $BITRISEIO_ANDROID_KEYSTORE_URL
    - BITRISEIO_ANDROID_KEYSTORE_PASSWORD: $BITRISEIO_ANDROID_KEYSTORE_PASSWORD
    - BITRISEIO_ANDROID_KEYSTORE_ALIAS: $BITRISEIO_ANDROID_KEYSTORE_ALIAS
    - BITRISEIO_ANDROID_KEYSTORE_PRIVATE_KEY_PASSWORD: $BITRISEIO_ANDROID_KEYSTORE_PRIVATE_KEY_PASSWORD
    steps:
    - activate-ssh-key:
        run_if: '{{getenv "SSH_RSA_PRIVATE_KEY" | ne ""}}'
    - git-clone: {}
    - script:
        title: "Select Java 11"
        run_if: $.IsCI
        inputs:
        - content: |-
            #!/usr/bin/env bash
            set -ex
            if [[ "$OSTYPE" == "linux-gnu"* ]]; then
              sudo update-alternatives --set javac /usr/lib/jvm/java-11-openjdk-amd64/bin/javac
              sudo update-alternatives --set java /usr/lib/jvm/java-11-openjdk-amd64/bin/java
              export JAVA_HOME="/usr/lib/jvm/java-11-openjdk-amd64"
              envman add --key JAVA_HOME --value "/usr/lib/jvm/java-11-openjdk-amd64" 
            elif [[ "$OSTYPE" == "darwin"* ]]; then
              jenv global 11
              export JAVA_HOME="$(jenv prefix)"
              envman add --key JAVA_HOME --value "$(jenv prefix)"
            fi
    - install-missing-android-tools:
        inputs:
        - gradlew_path: $PROJECT_LOCATION/gradlew
    - android-build:
        inputs:
        - project_location: $PROJECT_LOCATION
        - module: $MODULE
        - variant: $VARIANT
        - build_type: aab
```
2. Sign the App Bundle:
```yml
    - sign-apk:
        inputs:
        - use_apk_signer: true
```
3. Deploy the signed App Bundle to Bitrise:
```
    - deploy-to-bitrise-io: {}
```

## Configuration

### Inputs

| Parameter | Description | Required | Default |
| --- | --- | --- | --- |
| android_app | Path(s) to the build artifact file to sign (`.aab` or `.apk`). You can provide multiple build artifact file paths separated by `\|` character. Format examples: `/path/to/my/app.apk`; `/path/to/my/app1.apk\|/path/to/my/app2.apk`. | ✔️ | *$BITRISE_APK_PATH\n$BITRISE_AAB_PATH* |
| keystore_url | Keystore URL. For remote keystores you can provide any download location (example: `https://URL/TO/keystore.jks`). For local keystores provide file path url. (example: `file://PATH/TO/keystore.jks`). | ✔️ | *$BITRISEIO_ANDROID_KEYSTORE_URL* |
| keystore_password | Keystore password | ✔️ | *$BITRISEIO_ANDROID_KEYSTORE_PASSWORD* |
| keystore_alias | Key alias |  ✔️ | *$BITRISEIO_ANDROID_KEYSTORE_ALIAS* |
| private_key_password | Key password. If key password equals to keystore password (not recommended), you can leave it empty. | - | *$BITRISEIO_ANDROID_KEYSTORE_PRIVATE_KEY_PASSWORD* |
| page_align | If enabled, it tells zipalign to use memory page alignment for stored shared object files. Options: `automatic` (Enable page alignment for .so files, unless attribute *extractNativeLibs="true"* is set in the AndroidManifest.xml); `true`; `false` | ✔️ | `automatic` |
| use_apk_signer | Indicates if the signature should be done using apksigner instead of jarsigner. Options: `true`, `false`. | ✔️ | `false` |
| signer_scheme | APK Signature Scheme. `automatic` uses the values of --min-sdk-version and --max-sdk-version to decide which Signature Scheme to use. Options: `v2`, `v3`, `v4`, `automatic`. | ✔️ | `automatic` |
| debuggable_permitted | Whether to permit signing android:debuggable="true" APKs. Android disables some of its security protections for such apps. Options: `true`, `false`. | ✔️ | `true` |
| output_name | Name of the produced output artifact. By default the output name is *app-release-bitrise-signed*. Else it's the specified name. Do not add extensions. | - | "" |
| verbose_log | Enables verbose logging. Options: `true`, `false`. | ✔️ | `false` |
| apk_path | *deprecated* | - | - |

### Outputs

| Environment Variable | Description |
| --- | --- |
| BITRISE_SIGNED_APK_PATH | This output will include the path of the signed APK. If the build generates more than one APK this output will contain the last one's path. |
| BITRISE_SIGNED_APK_PATH_LIST | This output will include the paths of the generated APKs. If multiple APKs are provided for signing the output paths are separated with `\|` character, for example, `app-armeabi-v7a-debug.apk\|app-mips-debug.apk\|app-x86-debug.apk` |
| BITRISE_SIGNED_AAB_PATH | This output will include the path of the signed AAB. If the build generates more than one AAB this output will contain the last one's path. |
| BITRISE_SIGNED_AAB_PATH_LIST | This output will include the paths of the generated AABs. If multiple AABs are provided for signing the output paths are separated with `\|` character, for example, `app-armeabi-v7a-debug.aab\|app-mips-debug.aab\|app-x86-debug.aab` |
| BITRISE_APK_PATH | *deprecated* |
| BITRISE_AAB_PATH | *deprecated* |

## Contributing

We welcome [pull requests](https://github.com/bitrise-steplib/steps-sign-apk/pulls) and [issues](https://github.com/bitrise-steplib/steps-sign-apk/issues) against this repository. 

For pull requests, work on your changes in a forked repository and use the bitrise cli to [run your tests locally](https://devcenter.bitrise.io/bitrise-cli/run-your-first-build/).

### Creating your own steps

Follow [this guide](https://devcenter.bitrise.io/contributors/create-your-own-step/) if you would like to create your own step.
