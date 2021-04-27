# Android Sign ![Bitrise Build Status](https://app.bitrise.io/app/3b968e65d584db2a.svg?token=Yk1LUEjLZtIjeIW4OOZvKw&branch=master) [![Bitrise Step Version](https://shields.io/github/v/release/bitrise-steplib/steps-sign-apk?include_prereleases)](https://www.bitrise.io/integrations/steps/sign-apk) [![GitHub License](https://img.shields.io/badge/license-MIT-lightgrey.svg)](https://raw.githubusercontent.com/bitrise-steplib/steps-go-list/master/LICENSE) [![Bitrise Community](https://img.shields.io/badge/community-Bitrise%20Discuss-lightgrey)](https://discuss.bitrise.io/)

Signs your APK or Android App Bundle to be uploaded to the Google Play Store.

## Examples

### List packages in the working directory excluding vendor/*

```yml
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
    # If the Android keystore is configured in the workflow editor, BITRISEIO_ANDROID* envs will be set automatically
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
    - change-android-versioncode-and-versionname:
          run_if: $.IsCI
          inputs:
          - build_gradle_path: $PROJECT_LOCATION/$MODULE/build.gradle
    - android-build:
        inputs:
        - project_location: $PROJECT_LOCATION
        - module: $MODULE
        - variant: $VARIANT
        - build_type: aab
    - sign-apk:
        inputs:
        - use_apk_signer: true
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

For pull requests, work on your changes in a forked repository and use the bitrise cli to [run your tests locally](https://devcenter.bitrise.io/bitrise-cli/run-your-first-build/)

## Development environment

1. First, create *.bitrise.secrets.yml* with the contents below and fill out the blanks.
For testing purposes you can create a new keystore and use it all of the four cases covered.

```
envs:
# CI workflow
## Keystore password == key password
- ANDROID_SIGN_SAME_PASS_KEYSTORE_URL: file:///path/testme
- ANDROID_SIGN_SAME_PASS_KEYSTORE_PASSWORD: pass
- ANDROID_SIGN_SAME_PASS_KEYSTORE_ALIAS: alias
- ANDROID_SIGN_SAME_PASS_KEYSTORE_PRIVATE_KEY_PASSWORD: 
## Keystore password != key password
- ANDROID_SIGN_DIFF_PASS_KEYSTORE_URL: 
- ANDROID_SIGN_DIFF_PASS_KEYSTORE_PASSWORD: 
- ANDROID_SIGN_DIFF_PASS_KEYSTORE_ALIAS: 
- ANDROID_SIGN_DIFF_PASS_KEYSTORE_PRIVATE_KEY_PASSWORD: 
## Default alias ('mykey')
- ANDROID_SIGN_DEFAULT_ALIAS_KEYSTORE_URL: 
- ANDROID_SIGN_DEFAULT_ALIAS_KEYSTORE_PASSWORD: 
- ANDROID_SIGN_DEFAULT_ALIAS_KEYSTORE_ALIAS: 
- ANDROID_SIGN_DEFAULT_ALIAS_KEYSTORE_PRIVATE_KEY_PASSWORD: 
## Android Studio generated keystore
- ANDROID_SIGN_STUDIO_GEN_KEYSTORE_URL: 
- ANDROID_SIGN_STUDIO_GEN_KEYSTORE_PASSWORD: 
- ANDROID_SIGN_STUDIO_GEN_KEYSTORE_ALIAS: 
- ANDROID_SIGN_STUDIO_GEN_KEYSTORE_PRIVATE_KEY_PASSWORD: 

# Debug workflow
- BITRISE_APK_PATH: 
- BITRISEIO_ANDROID_KEYSTORE_URL: 
- BITRISEIO_ANDROID_KEYSTORE_PASSWORD: 
- BITRISEIO_ANDROID_KEYSTORE_ALIAS: 
- BITRISEIO_ANDROID_KEYSTORE_PRIVATE_KEY_PASSWORD: 
```

2. Run tests using:
- `bitrise run ci`    to run linters, Golang tests and end-to-end integration tests
- `bitrise run test`  to run e2e integration tests only
- `bitirse run debug` to check the Step in a specific workflow during development

### Creating your own steps

Follow [this guide](https://devcenter.bitrise.io/contributors/create-your-own-step/) if you would like to create your own step
