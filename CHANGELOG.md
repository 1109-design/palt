# Changelog

What's new in each PALT release. The section for a tag becomes its GitHub
release notes, which both apps show in "What's new".

## v2.4.1

- Fixed: the phone app showed an old version number and kept offering the update it already had.

## v2.4.0

- **Unlock the laptop with your phone's fingerprint**: the lock screen, admin prompts and sudo ask the phone; touch the sensor and you're in. Your password still works any time. Turn it on in the phone's device settings on the laptop.
- **Browse the phone's files from the laptop**: folders with picture and video previews, download, upload into any folder, new folder, delete.
- **Phone storage and memory**: used and free space split into photos, videos, audio, documents and apps, the biggest folders and files, SD card, RAM and swap.
- **Take a photo with the phone from the laptop**: the camera opens on the phone and the picture opens on the laptop.
- **Run commands from the phone**: Lock, Suspend, Mute, plus your own commands added on the laptop.
- **Laptop status on the phone**: locked or unlocked, CPU, memory, disk, temperature and uptime.
- **Proximity lock**: the laptop locks when the phone has been away for 2 minutes (optional).
- **Wake the laptop from the phone**: sleeping computers show on the phone's home screen with a Wake button (same network, from sleep).
- **Keep the laptop awake** while the phone is connected (optional).
- **Wi-Fi QR screenshots**: take a screenshot of a Wi-Fi QR code and the phone offers to send it to the laptop.
- Unread count on the Activity tab; a hint on the phone when Android blocks texts and calls for PALT.
- Fixed: the .deb now installs files owned by root.

## v2.3.0

- **New laptop look**: your devices up front on the home screen; each device page has a Control side (media, big action tiles, vitals) and an Activity side with chat bubbles and picture thumbnails.
- **Zoom the laptop app**: Ctrl + and Ctrl − (or Ctrl + scroll), Ctrl 0 to reset, or the zoom buttons in the top bar.
- **Laptop media in the phone's media controls**: whatever plays on the laptop shows in the phone's quick panel with play, pause, skip and seek, like GSConnect.
- **New now-playing player** on both apps: cover art glow, a wavy time bar, smooth play/pause animations.
- **Black look on the phone** with warm gradients on every page.
- **Netflix shows the real title**: install the PALT browser extension (in `browser-extension/`) in Chrome or Edge and the phone shows the show and episode with its artwork instead of just "Netflix".
- Fixed: the phone's volume slider now sets the laptop's volume (it could only mute before).
- Fixed: old YouTube artwork no longer sticks around on other videos.

## v2.2.0

- **New phone look**: your devices up front with their battery, network and the actions you use most; each device has a Control tab (media remote and big action tiles) and an Activity tab (a chat with picture thumbnails). Swipe between them.
- **Messages on the laptop**: read and send SMS through the phone, with your contacts. One-time codes get a Copy button in the notification.
- **Calls on the laptop**: call log, call anyone from the laptop, answer or decline with a live call bar. Works with both SIMs, for calls and texts.
- **Media remote**: control what's playing on the laptop from the phone and the other way round, volume included.
- **Share internet**: when one device is offline, the other offers its Wi-Fi or a hotspot.
- **You choose what each device gets**: clipboard and notifications only go to your own paired devices, with a switch per device.
- Fixed: "What's new" now shows the real release notes; startup errors are shown instead of quitting.

## v2.1.0

- **USB cable**: plug the phone in and turn on USB tethering — PALT works over the cable with no Wi-Fi or data, and the laptop keeps its own internet.
- **Fastest path**: PALT picks USB or Wi-Fi, whichever is faster, and switches within seconds when Wi-Fi goes or comes back.
- **See how you're connected**: USB cable, Same Wi-Fi or Via relay, on both apps.
- **Share Wi-Fi**: send the laptop's Wi-Fi to the phone in one tap; the laptop joins Wi-Fi from a QR code screenshot shared from the phone.
- **Clipboard from the phone, automatically**: copy anywhere on the phone, paste on the laptop (needs a one-time setup over USB).
- PALT now starts quietly in the tray at login.

## v2.0.0

- **Pair once, connect always**: scan a QR code to pair; paired devices link up by themselves and stay encrypted end to end.
- **Phone notifications on the laptop**, with reply and actions.
- **Calls**: see who's calling on the laptop; videos and music pause for calls.
- **Clipboard sync** between your devices.
- **Battery and network** of each device, both ways.
- **Find my phone / laptop**: ring and ping.
- Tray with your devices, start on login, Send with PALT in the file manager.
