### Examples

1. Build an Android App Bundle:

```yaml
workflows:
  release:
    envs:
    - PROJECT_LOCATION: .
    - MODULE: app
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

```yaml
    - sign-apk:
        inputs:
        - use_apk_signer: false
```

3. Deploy the signed App Bundle to Bitrise:

```yaml
    - deploy-to-bitrise-io: {}
```