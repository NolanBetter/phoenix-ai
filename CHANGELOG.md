# Changelog

Each section here becomes the release notes on GitHub, and the "What's new" text Phoenix shows when an update is ready.

## 0.4.1

- Fixed: making pictures and videos failed with "The engine stopped". Phoenix was downloading an older version of the picture and video engine than the one it was built for. The right version downloads by itself a few seconds after Phoenix starts.
- When an engine fails, Phoenix now shows the actual error instead of the last line of the engine's help text.

## 0.4.0

- **Make pictures and videos.** Download Stable Diffusion XL (pictures) or Wan 2.2 (short videos) on the Models page, pick it in the model menu, and describe what you want. Choose square, wide or tall, and 2 to 5 seconds for videos. Add a picture with **+** to redo it or bring it to life. Save what you make with the download button.
- **Talk to Phoenix.** The voice chat button opens a conversation where Phoenix listens, answers, and talks back. Tap the circle to interrupt it.
- **Speak instead of typing** with the microphone button, and have any answer **read aloud** with the speaker button. Pick the voice and speed in Settings.
- **Add audio files** with **+**: MP3, WAV, M4A and more. Phoenix writes them down so you can ask about them. With Speaker detection downloaded, it also marks who said what.
- **Qwen 3 Coder Next**, the best open coding model that runs at home. It's big: about 40 GB of memory.
- Phoenix asks before a download that would use more than a quarter of your free space.
- Speech recognition, speaker detection and the Kokoro voice don't count toward the 5-model limit.
- Fixed: downloads that failed showed as paused, and a stray "null" in Settings.

## 0.3.0

- Add files to a message with the **+** button, by dropping them on the window, or by pasting a screenshot: pictures, PDFs, Word documents, and text or code files.
- Models marked **Sees images** can look at your pictures. Their small image-support file downloads with them. For a model you already have, choose **Add it** on the Models page.
- HTML and SVG code in answers now shows a live **Preview** when the answer is done. Switch between Preview and Code, or open it full screen.
- The **Copy** button on code stays at the top while you scroll through long code.
- Windows notifications when an answer or download finishes while you're in another window. Click one to jump back. You can turn them off in Settings.
- A new title bar with minimize, maximize and close. Phoenix opens to fill the screen the first time, remembers its size after that, and never opens bigger than the screen.

## 0.2.0

- Installs like a normal app: **Phoenix AI** in the Start menu, on your desktop, and in Settings > Apps for uninstalling. No administrator prompt.
- Updates itself from GitHub. New versions download in the background and install when you restart or close Phoenix.
- A redder look, with scrollbars that match.
- Rename and delete chats from the ⋯ button next to each chat, or by right-clicking it.
- A spinner shows which chat is writing an answer, and a dot marks new answers you haven't read yet.
- Phoenix keeps up to 10 chats and 5 models.
- Clearer Delete button for models.

## 0.1.0

- First version: download open-source models and chat with them on your PC.
