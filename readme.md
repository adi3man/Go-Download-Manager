# Go Download Manager (GDM)

**Go Download Manager (GDM)** is a lightweight, fast, and modern desktop download manager. Built with **Go** and powered by the **Fyne v2** GUI framework, GDM offers an intuitive interface to handle local downloads while supporting seamless integration with web browsers via a background REST API.

---

##  Features

*   **Full Download Control (Pause & Resume)**: Pause downloads at any time and resume them from where they left off using HTTP Range Requests, avoiding the need to restart from scratch.
*   **Refresh Download Address**: Update the download URL directly from the right-click menu if a link expires, and instantly resume your progress.
*   **Contextual Right-Click Menu**: Access essential actions (*Pause, Resume, Refresh URL, and Delete Task*) precisely at your cursor position.
*   **Real-Time Information**: Clean list view displaying Filename, Size (Progress / Total), Status & Speed (*real-time speed tracking*), and Date Added.
*   **Clear Finished Tasks**: Declutter your download list by removing completed or failed tasks with a single click on the **"Clear Finished"** button.
*   **Bilingual Support**: Dynamically switch languages between **English** and **Bahasa Indonesia** instantly without restarting the application.
*   **Smart Category Routing**: Automatically organizes downloaded files into subfolders based on file extension (e.g., `.zip` moves to *Compressed*, `.mp4` to *Video*).
*   **Background API Listener**: Runs a lightweight server on port `18080` to receive new download commands directly from browser extensions via a simple REST API.

---

##  Prerequisites

Before running or building GDM, make sure you have the following installed:

1.  **Go** (version 1.18 or newer).
2.  A graphics driver with OpenGL support (required by Fyne).
    *   **Windows**: MSYS2 / Mingw-w64 (for CGO compilation).
    *   **Linux**: `libgl1-mesa-dev`, `xorg-dev`, and standard GCC tools.
    *   **macOS**: Xcode Command Line Tools.

---

##  Getting Started

### 1. Clone the Repository
```bash
git clone [https://github.com/your-username/go-download-manager.git](https://github.com/your-username/go-download-manager.git)
cd go-download-manager
