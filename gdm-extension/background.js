const GDM_URL = "http://localhost:18080/download";

// Mencegat unduhan saat baru dibuat
chrome.downloads.onCreated.addListener((downloadItem) => {
    // Abaikan jika bukan URL valid, atau jika itu adalah blob/data internal browser
    if (!downloadItem.url ||
        downloadItem.url.startsWith("blob:") ||
        downloadItem.url.startsWith("data:") ||
        downloadItem.url.startsWith("chrome-extension://")) {
        return;
        }

        // Jika unduhan ini berasal dari request yang kita buat sendiri ke localhost, jangan dicegat (mencegah loop)
        if (downloadItem.url.includes("localhost:18080") || downloadItem.url.includes("127.0.0.1:18080")) {
            return;
        }

        console.log("Mencegat unduhan:", downloadItem.url);

        // 1. Langsung batalkan unduhan bawaan browser
        chrome.downloads.cancel(downloadItem.id, () => {
            // Hapus dari riwayat unduhan agar bersih
            chrome.downloads.erase({ id: downloadItem.id });
        });

        // 2. Kirim URL-nya ke Go Download Manager
        sendToGDM(downloadItem.url);
});

// Fitur Klik Kanan sebagai cadangan
chrome.runtime.onInstalled.addListener(() => {
    chrome.contextMenus.removeAll(() => {
        chrome.contextMenus.create({
            id: "gdm-download",
            title: "Unduh dengan Go Download Manager",
            contexts: ["link"]
        });
    });
});

chrome.contextMenus.onClicked.addListener((info, tab) => {
    if (info.menuItemId === "gdm-download" && info.linkUrl) {
        sendToGDM(info.linkUrl);
    }
});

// Fungsi untuk mengirim data ke Go backend
function sendToGDM(targetUrl) {
    fetch(GDM_URL, {
        method: "POST",
        headers: {
            "Content-Type": "application/json"
        },
        body: JSON.stringify({ url: targetUrl })
    })
    .then(response => {
        console.log("Berhasil dikirim ke GDM:", response);
    })
    .catch(err => {
        console.error("Gagal mengirim ke GDM. Pastikan aplikasi Go Anda sudah dijalankan! Error:", err);
    });
}
