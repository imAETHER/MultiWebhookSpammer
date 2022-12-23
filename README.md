## <p align="center">🧪 MultiWebhook Spammer</p>

<img src="https://user-images.githubusercontent.com/36291026/209234406-6d668e79-2abb-4ccc-aa5a-a2dfb92cd88e.png">

My multi-webhook spammer now made in Go with a simple UI!

# Features

- Manual Input
  - Paste in multiple webhooks in a textbox and press <kbd>CTRL+S</kbd> to start spamming.
- Load from file
  - Load many webhooks from a file, each separated with a newline.
- Live webhook status and info display
  - Shows the status of the current webhooks with colors:
    - 🔴 deleted/invalid
    - 🟣 ratelimited
    - 🟢 valid and alive

## TODO

- [ ] Dynamic alignment
- [x] Spam Message customization

## Known bugs

- Webhooks are only stacked horizontally, will overflow if the window size is too small
