# 📊 godu - Quick Disk Space Visualizer

[![Download](https://img.shields.io/badge/Download-Get%20godu-blue?style=for-the-badge)](https://github.com/Lime0moss/godu/releases)

---

## 📃 What is godu?

godu is a tool to help you see what files and folders use up space on your computer. It runs in the terminal (the black or white window where you can type commands) and shows your disk usage in an easy way. You can view your data as a list, a tree, or pictures called treemaps. It also lets you delete items safely and save a report as a JSON file if you want.

You do not need any special skills to use godu. It works on many systems like Windows, macOS, and Linux.

---

## 💻 System Requirements

Before installing godu, make sure your computer meets these needs:

- **Operating System:** Windows 10 or later, macOS 10.13 or later, or a Linux distribution.
- **Processor:** Any modern CPU (Intel, AMD, or equivalent).
- **Memory:** At least 2 GB of RAM.
- **Disk Space:** At least 100 MB free space for installing and running.
- **Terminal Access:** You will need to open a terminal or command prompt window.
  
No additional software is required. godu is a self-contained program built with Go, so you don’t need to install Go or any other tools.

---

## 🚀 Getting Started

This guide shows you how to download, install, and start using godu. You don’t have to write any code or use complex commands.

### Step 1: Download godu

Click the big blue button at the top or go to the [godu Releases Page](https://github.com/Lime0moss/godu/releases). This page holds all versions of godu.

Look for the version matching your computer's operating system:

- Windows files usually end with `.exe`
- macOS files might end with `.dmg` or `.tar.gz`
- Linux files often end with `.tar.gz` or `.AppImage`

Download the file that fits your system.

### Step 2: Install godu

- **Windows:**

  Find the `.exe` file you downloaded (usually in your Downloads folder). Double-click it and follow the installer steps on the screen.  
  If it's a zip file, right-click and choose "Extract All," then open the folder and run `godu.exe`.

- **macOS:**

  If you have a `.dmg` file, open it and drag the godu app into your Applications folder.  
  If it’s a `.tar.gz`, double-click to extract it, then move the godu file somewhere you can find it.

- **Linux:**

  Extract the `.tar.gz` archive with a command like `tar -xvzf godu-version-linux.tar.gz` using the terminal. You can move the extracted files to a folder like `/usr/local/bin` for easier access.

### Step 3: Open your Terminal or Command Prompt

- On Windows, press the Windows key, type `cmd`, and press Enter.
- On macOS, open "Terminal" from Applications > Utilities.
- On Linux, open your preferred terminal emulator.

### Step 4: Run godu

In your terminal, type:

```bash
godu
```

and press Enter.

godu will scan your computer's main drive and show your disk usage in a tree view. You can navigate through folders using your keyboard.

---

## 🔍 How to Use godu

### Navigating Views

- **Tree view:** Shows folders and files in a list you can expand and collapse.
- **Treemap:** Displays colored blocks sized by disk space for a visual summary.
- **File type breakdown:** Lists which types of files (like images, videos, documents) take up the most space.

Switch views with the keyboard commands shown at the bottom of the screen.

### Safe Deletion

You can delete files and folders without leaving godu. Select an item and press the delete key. godu asks for confirmation first, so you do not remove anything by accident.

### Export Data

You can create a JSON report of your disk usage for analysis or sharing. Press the export key (usually `e`) and choose where to save the file.

---

## 📥 Download & Install

To get godu, please visit the Releases Page:

[Download godu from Releases](https://github.com/Lime0moss/godu/releases)

Follow the instructions in the "Getting Started" section above according to your operating system.

---

## 🛠 Features

- Fast scanning of disk usage.
- Interactive terminal interface.
- Multiple views: tree, treemap, file types.
- Safe deletion of files and folders.
- Export disk usage data as JSON.
- Works on Windows, macOS, and Linux.
- Small and easy to install.
- Built with Go for speed and reliability.

---

## 📚 Troubleshooting & Tips

- If godu does not run, check if the file you downloaded matches your OS.
- Make sure your terminal is working and you can run other commands.
- On macOS and Linux, you might need to give permission to run godu with:

  ```bash
  chmod +x godu
  ```

- Use arrow keys to navigate the views.
- Press `q` to quit godu at any time.
- For support, you can open an issue on the GitHub repository.

---

## 📂 Further Information

You can find more details about godu, report bugs, or request features on its [GitHub repository](https://github.com/Lime0moss/godu).

---

## 🎯 Topics

godu is related to these areas:

- bubbletea
- cli (command-line interface)
- disk-analyzer
- disk-usage
- go / golang programming language
- ncdu (similar tools)
- terminal applications
- treemap representations
- tui (text user interface)