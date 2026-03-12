# Firebase Setup Guide for ATAP Mobile

This guide walks through configuring Firebase Cloud Messaging (FCM) for push notifications in the ATAP mobile app.

## Prerequisites

- A Google account
- Access to [Firebase Console](https://console.firebase.google.com)
- An Apple Developer account (for iOS push notifications)
- The ATAP mobile app source code

## 1. Create a Firebase Project

1. Go to [Firebase Console](https://console.firebase.google.com)
2. Click **Add project**
3. Enter project name (e.g., "ATAP" or "ATAP Dev")
4. Optionally enable Google Analytics (not required for push)
5. Click **Create project**

## 2. Add Android App

1. In the Firebase Console, go to **Project Settings** (gear icon)
2. Click **Add app** and select **Android**
3. Enter the Android package name: `dev.atap.atap`
4. Enter app nickname: "ATAP Android"
5. Click **Register app**
6. Download `google-services.json`
7. Place it at `mobile/android/app/google-services.json`

**Important:** Do NOT commit `google-services.json` to git. It is listed in `.gitignore`.

## 3. Add iOS App

1. In the Firebase Console, go to **Project Settings**
2. Click **Add app** and select **iOS**
3. Enter the iOS bundle ID (check `mobile/ios/Runner.xcodeproj/project.pbxproj` for `PRODUCT_BUNDLE_IDENTIFIER`)
4. Enter app nickname: "ATAP iOS"
5. Click **Register app**
6. Download `GoogleService-Info.plist`
7. Place it at `mobile/ios/Runner/GoogleService-Info.plist`
8. **In Xcode:** Right-click the `Runner` folder, select "Add Files to Runner...", select `GoogleService-Info.plist`, and ensure "Runner" target is checked

**Important:** Do NOT commit `GoogleService-Info.plist` to git. It is listed in `.gitignore`.

## 4. Upload APNs Key (Required for iOS Push)

iOS push notifications require an APNs authentication key from Apple:

1. Go to [Apple Developer Portal](https://developer.apple.com/account/resources/authkeys/list)
2. Click the **+** button to create a new key
3. Enter a name (e.g., "ATAP Push Key")
4. Check **Apple Push Notifications service (APNs)**
5. Click **Continue**, then **Register**
6. Download the `.p8` key file and note the **Key ID**
7. Note your **Team ID** (visible in Apple Developer portal top-right)

Then upload to Firebase:

1. In Firebase Console, go to **Project Settings** > **Cloud Messaging**
2. Under **iOS app configuration**, click **Upload** next to "APNs Authentication Key"
3. Upload the `.p8` file
4. Enter the **Key ID** and **Team ID**

## 5. Platform Server Configuration

The ATAP platform server needs Firebase credentials to send push notifications:

1. In Firebase Console, go to **Project Settings** > **Service accounts**
2. Click **Generate new private key**
3. Download the JSON service account file
4. Set the environment variable on the platform server:

```bash
export GOOGLE_APPLICATION_CREDENTIALS="/path/to/service-account-key.json"
```

The platform server will automatically initialize Firebase Admin SDK when this environment variable is set.

## 6. Verify Setup

After completing the above steps:

1. Run the app on a physical device (push notifications do not work on iOS simulator)
2. Accept the notification permission prompt
3. Check the logs for "FCM token registered" message
4. Send a test notification from Firebase Console > **Messaging** > **Create your first campaign**

## Notes

- **Without `google-services.json` and `GoogleService-Info.plist`, the app will not compile.** This is intentional -- Firebase must be configured before the app can run.
- The `google-services.json` and `GoogleService-Info.plist` files contain project-specific configuration but no secret keys. However, they should still be kept out of version control as a best practice.
- Push notifications require a physical device. iOS Simulator does not support push notifications.
- The background message handler runs in a separate Dart isolate, so it cannot access app state directly.
