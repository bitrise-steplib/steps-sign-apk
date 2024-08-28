# Android Sign

[![Step changelog](https://shields.io/github/v/release/bitrise-steplib/steps-sign-apk?include_prereleases&label=changelog&color=blueviolet)](https://github.com/bitrise-steplib/steps-sign-apk/releases)

Signs your APK or Android App Bundle before uploading it to Google Play Store.

<details>
<summary>Description</summary>

Once you have uploaded your keystore file and provided your keystore credentials on the **Code Signing** tab of the **App Settings** page, the **Android Sign** Step signs your APK digitally.
Bitrise assigns Environment Variables to the uploaded file and credentials, and uses those in the respective fields of the **Android Sign** Step.
Once the Step runs, it produces a signed APK or App Bundle which will be used as the input value of the **App file path** field in the **Google Play Deploy** Step.

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
</details>

## 🧩 Get started

Add this step directly to your workflow in the [Bitrise Workflow Editor](https://devcenter.bitrise.io/steps-and-workflows/steps-and-workflows-index/).

You can also run this step directly with [Bitrise CLI](https://github.com/bitrise-io/bitrise).

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

## ⚙️ Configuration

<details>
<summary>Inputs</summary>

| Key | Description | Flags | Default |
| --- | --- | --- | --- |
| `android_app` | Path(s) to the build artifact file to sign (`.aab` or `.apk`).  You can provide multiple build artifact file paths separated by `\|` character.  Format examples:  - `/path/to/my/app.apk` - `/path/to/my/app1.apk\|/path/to/my/app2.apk\|/path/to/my/app3.apk`  - `/path/to/my/app.aab` - `/path/to/my/app1.aab\|/path/to/my/app2.apk\|/path/to/my/app3.aab` | required | `$BITRISE_APK_PATH\n$BITRISE_AAB_PATH` |
| `keystore_url` | For remote keystores you can provide any download location (e.g. `https://URL/TO/keystore.jks`). For local keystores provide file path url. (e.g. `file://PATH/TO/keystore.jks`). | required, sensitive | `$BITRISEIO_ANDROID_KEYSTORE_URL` |
| `keystore_password` | Matching password to `keystore_url`. Do not confuse this with `key_password`! | required, sensitive | `$BITRISEIO_ANDROID_KEYSTORE_PASSWORD` |
| `keystore_alias` | Alias of key inside `keystore_url`. | required, sensitive | `$BITRISEIO_ANDROID_KEYSTORE_ALIAS` |
| `private_key_password` | If key password equals to keystore password (not recommended), you can leave it empty. Otherwise specify the private key password.  | sensitive | `$BITRISEIO_ANDROID_KEYSTORE_PRIVATE_KEY_PASSWORD` |
| `page_align` | If enabled, it tells zipalign to use memory page alignment for stored shared object files.  - `automatic`: Enable page alignment for .so files, unless atribute `extractNativeLibs="true"` is set in the AndroidManifest.xml - `true`: Enable memory page alignment for .so files - `false`: Disable memory page alignment for .so files  | required | `automatic` |
| `use_apk_signer` | Indicates if the signature should be done using `apksigner` instead of `jarsigner`. | required | `false` |
| `signer_scheme` | If set, enforces which Signature Scheme should be used by the project.  - `automatic`: The tool uses the values of `--min-sdk-version` and `--max-sdk-version` to decide when to apply this Signature Scheme. - `v2`: Sets `--v2-signing-enabled` true, and determines whether apksigner signs the given APK package using the APK Signature Scheme v2. - `v3`: Sets `--v3-signing-enabled` true, and determines whether apksigner signs the given APK package using the APK Signature Scheme v3. - `v4`: Sets `--v4-signing-enabled` true, and determines whether apksigner signs the given APK package using the APK Signature Scheme v4. This scheme produces a signature in an separate file (apk-name.apk.idsig). If true and the APK is not signed, then a v2 or v3 signature is generated based on the values of `--min-sdk-version` and `--max-sdk-version`.  | required | `automatic` |
| `debuggable_permitted` | Whether to permit signing `android:debuggable="true"` APKs. Android disables some of its security protections for such apps.  | required | `true` |
| `output_name` | If empty, then the output name is `app-release-bitrise-signed`. Otherwise, it's the specified name. Do not add the file extension here.  |  |  |
| `verbose_log` | Enable verbose logging? | required | `false` |
| `apk_path` | __This input is deprecated and will be removed on 20 August 2019, use `App file path` input instead!__  Path(s) to the build artifact file to sign (`.aab` or `.apk`).  You can provide multiple build artifact file paths separated by `\|` character.  Deprecated, use `android_app` instead.  Format examples:  - `/path/to/my/app.apk` - `/path/to/my/app1.apk\|/path/to/my/app2.apk\|/path/to/my/app3.apk`  - `/path/to/my/app.aab` - `/path/to/my/app1.aab\|/path/to/my/app2.apk\|/path/to/my/app3.aab` |  |  |
</details>

<details>
<summary>Outputs</summary>

| Environment Variable | Description |
| --- | --- |
| `BITRISE_SIGNED_APK_PATH` | This output will include the path of the signed APK. If the build generates more than one APK this output will contain the last one's path. |
| `BITRISE_SIGNED_APK_PATH_LIST` | This output will include the paths of the generated APKs If multiple APKs are provided for signing the output paths are separated with `\|` character, for example, `app-armeabi-v7a-debug.apk\|app-mips-debug.apk\|app-x86-debug.apk` |
| `BITRISE_SIGNED_AAB_PATH` | This output will include the path of the signed AAB. If the build generates more than one AAB this output will contain the last one's path. |
| `BITRISE_SIGNED_AAB_PATH_LIST` | This output will include the paths of the generated AABs. If multiple AABs are provided for signing the output paths are separated with `\|` character, for example, `app-armeabi-v7a-debug.aab\|app-mips-debug.aab\|app-x86-debug.aab` |
| `BITRISE_APK_PATH` | This output will include the path(s) of the signed APK(s). If multiple APKs are provided for signing the output paths are separated with `\|` character, for example, `app-armeabi-v7a-debug.apk\|app-mips-debug.apk\|app-x86-debug.apk` |
| `BITRISE_AAB_PATH` | This output will include the path(s) of the signed AAB(s). If multiple AABs are provided for signing the output paths are separated with `\|` character, for example, `app-armeabi-v7a-debug.aab\|app-mips-debug.aab\|app-x86-debug.aab` |
</details>

## 🙋 Contributing

We welcome [pull requests](https://github.com/bitrise-steplib/steps-sign-apk/pulls) and [issues](https://github.com/bitrise-steplib/steps-sign-apk/issues) against this repository.

For pull requests, work on your changes in a forked repository and use the Bitrise CLI to [run step tests locally](https://devcenter.bitrise.io/bitrise-cli/run-your-first-build/).

Learn more about developing steps:

- [Create your own step](https://devcenter.bitrise.io/contributors/create-your-own-step/)
- [Testing your Step](https://devcenter.bitrise.io/contributors/testing-and-versioning-your-steps/)
