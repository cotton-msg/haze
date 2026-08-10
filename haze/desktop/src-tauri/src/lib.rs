mod tray;

use std::sync::atomic::{AtomicBool, Ordering};

use tauri::{Manager, WindowEvent};
use tauri_plugin_global_shortcut::{Code, GlobalShortcutExt, Modifiers, Shortcut, ShortcutState};

struct Quitting(AtomicBool);

struct RegisteredShortcut(Shortcut);

#[tauri::command]
fn get_platform() -> String {
    format!("{} {}", std::env::consts::OS, std::env::consts::ARCH)
}

#[tauri::command]
fn show_notification(app: tauri::AppHandle, title: String, body: String) {
    use tauri_plugin_notification::NotificationExt;
    app.notification()
        .builder()
        .title(&title)
        .body(&body)
        .show()
        .ok();
}

#[tauri::command]
fn open_external_url(url: String) {
    open::that(&url).ok();
}

#[tauri::command]
fn get_system_info() -> serde_json::Value {
    serde_json::json!({
        "os": std::env::consts::OS,
        "arch": std::env::consts::ARCH,
        "hostname": hostname::get().map(|h| h.to_string_lossy().to_string()).unwrap_or_default(),
    })
}

#[tauri::command]
fn minimize_window(window: tauri::Window) {
    let _ = window.minimize();
}

#[tauri::command]
fn toggle_maximize_window(window: tauri::Window) -> bool {
    if window.is_maximized().unwrap_or(false) {
        let _ = window.unmaximize();
    } else {
        let _ = window.maximize();
    }
    window.is_maximized().unwrap_or(false)
}

#[tauri::command]
fn close_window(window: tauri::Window) {
    let _ = window.close();
}

#[tauri::command]
fn hide_window(app: tauri::AppHandle) {
    tray::hide_main_window(&app);
}

#[tauri::command]
fn show_window(app: tauri::AppHandle) {
    tray::show_main_window(&app);
}

#[tauri::command]
async fn check_for_updates(
    app: tauri::AppHandle,
) -> Result<serde_json::Value, String> {
    use tauri_plugin_updater::UpdaterExt;

    match app.updater() {
        Ok(updater) => match updater.check().await {
            Ok(Some(update)) => {
                let version = update.version.clone();
                match update.download_and_install(|_event| {}, || {}).await {
                    Ok(true) => Ok(serde_json::json!({
                        "status": "installed",
                        "version": version,
                    })),
                    Ok(false) => Ok(serde_json::json!({
                        "status": "none",
                    })),
                    Err(err) => Err(err.to_string()),
                }
            }
            Ok(None) => Ok(serde_json::json!({ "status": "none" })),
            Err(err) => Err(err.to_string()),
        },
        Err(err) => Err(err.to_string()),
    }
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    let shortcut = Shortcut::new(Some(Modifiers::CONTROL | Modifiers::SHIFT), Code::KeyM);

    tauri::Builder::default()
        .manage(Quitting(AtomicBool::new(false)))
        .plugin(tauri_plugin_shell::init())
        .plugin(tauri_plugin_notification::init())
        .plugin(tauri_plugin_single_instance::init(|app, _args, _cwd| {
            tray::show_main_window(app);
        }))
        .plugin(
            tauri_plugin_global_shortcut::Builder::new()
                .with_handler(move |app, _shortcut, event| {
                    if event.state() == ShortcutState::Pressed {
                        let is_visible = app
                            .get_webview_window("main")
                            .map(|w| w.is_visible().unwrap_or(false))
                            .unwrap_or(false);
                        if is_visible {
                            tray::hide_main_window(app);
                        } else {
                            tray::show_main_window(app);
                        }
                    }
                })
                .build(),
        )
        .plugin(tauri_plugin_updater::Builder::new().build())
        .invoke_handler(tauri::generate_handler![
            get_platform,
            show_notification,
            open_external_url,
            get_system_info,
            minimize_window,
            toggle_maximize_window,
            close_window,
            hide_window,
            show_window,
            check_for_updates,
        ])
        .setup(|app| {
            tray::create_tray(app.handle())?;
            let registered = app.global_shortcut().register(shortcut)?;
            app.manage(RegisteredShortcut(registered));
            Ok(())
        })
        .on_window_event(|window, event| {
            if let WindowEvent::CloseRequested { api, .. } = event {
                let quitting = window
                    .app_handle()
                    .try_state::<Quitting>()
                    .is_some_and(|q| q.0.load(Ordering::SeqCst));
                if quitting {
                    return;
                }
                api.prevent_close();
                let _ = window.hide();
            }
        })
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
