package com.cyledger.android;

import android.app.Activity;
import android.app.AlertDialog;
import android.content.Intent;
import android.content.pm.ApplicationInfo;
import android.content.res.Configuration;
import android.graphics.Color;
import android.net.Uri;
import android.os.Bundle;
import android.os.Handler;
import android.os.Looper;
import android.util.Base64;
import android.view.Gravity;
import android.view.View;
import android.view.WindowInsets;
import android.view.WindowInsetsController;
import android.webkit.CookieManager;
import android.webkit.ValueCallback;
import android.webkit.WebChromeClient;
import android.webkit.WebResourceError;
import android.webkit.WebResourceRequest;
import android.webkit.WebResourceResponse;
import android.webkit.WebSettings;
import android.webkit.WebView;
import android.webkit.WebViewClient;
import android.widget.Button;
import android.widget.LinearLayout;
import android.widget.TextView;
import android.widget.Toast;

import org.json.JSONObject;
import org.json.JSONTokener;

import java.io.ByteArrayOutputStream;
import java.io.ByteArrayInputStream;
import java.io.File;
import java.io.FileInputStream;
import java.io.FileOutputStream;
import java.io.InputStream;
import java.io.OutputStream;
import java.net.HttpURLConnection;
import java.net.URL;
import java.nio.charset.StandardCharsets;
import java.security.KeyStore;
import java.security.cert.X509Certificate;
import java.util.Locale;
import java.util.Collections;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.zip.ZipEntry;
import java.util.zip.ZipInputStream;

import javax.net.ssl.TrustManager;
import javax.net.ssl.TrustManagerFactory;
import javax.net.ssl.X509TrustManager;

public final class MainActivity extends Activity {
    private static final Object START_LOCK = new Object();
    private static boolean backendStarted;
    private static volatile String startupError;
    private static final int PICK_FILE = 71;
    private static final int SAVE_FILE = 72;
    private static final int MAX_DOWNLOAD = 20 * 1024 * 1024;
    private final Handler handler = new Handler(Looper.getMainLooper());
    private final ExecutorService io = Executors.newSingleThreadExecutor();
    private LinearLayout root;
    private WebView web;
    private String origin;
    private ValueCallback<Uri[]> uploadCallback;
    private byte[] pendingDownload;
    private boolean loaded;
    private LocalBridge local;
    private android.webkit.GeolocationPermissions.Callback geoCallback;
    private String geoOrigin;
    private boolean destroyed;
    private Boolean appearanceDark;
    private android.window.OnBackInvokedCallback backCallback;

    @Override public void onCreate(Bundle state) {
        super.onCreate(state);
        try { MaintenanceActivity.recover(getFilesDir()); } catch(Exception error){ finish(); return; }
        local=new LocalBridge(this);
        int port = getPackageName().endsWith(".qa") ? 18762 : 18761;
        origin = "http://127.0.0.1:" + port;
        root = new LinearLayout(this);
        root.setOrientation(LinearLayout.VERTICAL);
        setContentView(root);
        applyInsets();
        if (android.os.Build.VERSION.SDK_INT >= 33) {
            backCallback = this::onBackPressed;
            getOnBackInvokedDispatcher().registerOnBackInvokedCallback(
                android.window.OnBackInvokedDispatcher.PRIORITY_DEFAULT, backCallback);
        }
        showStatus("正在打开手机账本…", false);
        synchronized (START_LOCK) {
            if (!backendStarted) {
                backendStarted = true;
                Thread nativeThread = new Thread(() -> {
                    try {
                        File directory = new File(getFilesDir(), "ledger");
                        if (!directory.isDirectory() && !directory.mkdirs()) throw new Exception("private directory");
                        prepareAssets(directory);
                        prepareTrustStore(directory);
                        startupError = NativeBridge.start(directory.getAbsolutePath(), port);
                    } catch (Throwable error) {
                        startupError = "手机账本服务启动失败。请保留应用数据，关闭后重新打开。";
                    }
                }, "CYLedger-local-service");
                nativeThread.setDaemon(true);
                nativeThread.start();
            }
        }
        io.execute(() -> waitForBackend(state));
    }

    private void applyInsets() {
        if (android.os.Build.VERSION.SDK_INT >= 30) {
            getWindow().setDecorFitsSystemWindows(false);
            root.setOnApplyWindowInsetsListener((view, insets) -> {
                android.graphics.Insets bars = insets.getInsets(WindowInsets.Type.systemBars() | WindowInsets.Type.displayCutout());
                android.graphics.Insets keyboard = insets.getInsets(WindowInsets.Type.ime());
                view.setPadding(bars.left, bars.top, bars.right, Math.max(bars.bottom, keyboard.bottom));
                return insets;
            });
            WindowInsetsController controller = getWindow().getInsetsController();
            if (controller != null) {
                boolean dark = appearanceDark != null ? appearanceDark : (getResources().getConfiguration().uiMode & Configuration.UI_MODE_NIGHT_MASK) == Configuration.UI_MODE_NIGHT_YES;
                int flags = WindowInsetsController.APPEARANCE_LIGHT_STATUS_BARS | WindowInsetsController.APPEARANCE_LIGHT_NAVIGATION_BARS;
                controller.setSystemBarsAppearance(dark ? 0 : flags, flags);
            }
        }
        root.requestApplyInsets();
    }

    private void showStatus(String message, boolean retry) {
        root.removeAllViews();
        root.setBackgroundColor(Color.rgb(247, 244, 237));
        TextView label = new TextView(this);
        label.setText(message);
        label.setTextColor(Color.rgb(40, 53, 49));
        label.setTextSize(18);
        label.setGravity(Gravity.CENTER);
        root.addView(label, new LinearLayout.LayoutParams(-1, 0, 1));
        if (retry) {
            Button button = new Button(this);
            button.setText("重新连接手机账本");
            button.setOnClickListener(view -> io.execute(() -> waitForBackend(null)));
            root.addView(button, new LinearLayout.LayoutParams(-1, -2));
        }
    }

    private void waitForBackend(Bundle state) {
        for (int attempt = 0; attempt < 180 && !destroyed; attempt++) {
            if (startupError != null) {
                handler.post(() -> { if (!destroyed) showStatus(startupError, true); });
                return;
            }
            HttpURLConnection connection = null;
            try {
                connection = (HttpURLConnection) new URL(origin + "/healthz.json").openConnection();
                connection.setConnectTimeout(500);
                connection.setReadTimeout(1000);
                if (connection.getResponseCode() == 200) {
                    handler.post(() -> { if (!destroyed) openLedger(state); });
                    return;
                }
            } catch (Exception ignored) {
                // Native startup can take a few seconds on the first installation.
            } finally {
                if (connection != null) connection.disconnect();
            }
            try { Thread.sleep(500); } catch (InterruptedException error) { return; }
        }
        handler.post(() -> { if (!destroyed) showStatus("账本暂时没有响应。数据仍保存在手机，请关闭后重新打开。", true); });
    }

    private void openLedger(Bundle state) { local.ensureUnlocked(()->openLedgerUnlocked(state)); }
    private void openLedgerUnlocked(Bundle state) {
        loaded = true;
        root.removeAllViews();
        if (web == null) {
            web = new WebView(this);
            local.attach(web);
            web.addJavascriptInterface(local,"CYLedgerLocal");
            web.addJavascriptInterface(new ReminderBridge(this, web), "CYLedgerReminders");
            web.addJavascriptInterface(new AppearanceBridge(), "CYLedgerAppearance");
            WebView.setWebContentsDebuggingEnabled((getApplicationInfo().flags & ApplicationInfo.FLAG_DEBUGGABLE) != 0);
            WebSettings settings = web.getSettings();
            settings.setJavaScriptEnabled(true);
            settings.setDomStorageEnabled(true);
            settings.setCacheMode(WebSettings.LOAD_NO_CACHE);
            settings.setAllowFileAccess(false);
            settings.setAllowContentAccess(true);
            settings.setMixedContentMode(WebSettings.MIXED_CONTENT_NEVER_ALLOW);
            settings.setMediaPlaybackRequiresUserGesture(true);
            CookieManager.getInstance().setAcceptThirdPartyCookies(web, false);
            web.setWebViewClient(new WebViewClient() {
                @Override public WebResourceResponse shouldInterceptRequest(WebView view, WebResourceRequest request) {
                    if (!isLocal(request.getUrl().toString())) return new WebResourceResponse("text/plain","UTF-8",403,"Forbidden",Collections.emptyMap(),new ByteArrayInputStream(new byte[0]));
                    // Credentials enter only this app's WebView, through JNI.
                    // No HTTP session endpoint or public JavaScript bridge exists.
                    if (!request.isForMainFrame() || !isLocal(request.getUrl().toString())
                        || !"/personal".equals(request.getUrl().getPath())) return null;
                    try (InputStream stream = new FileInputStream(new File(getFilesDir(), "ledger/public/mobile.html"))) {
                        // Refresh after every reload: the Vue token refresh can
                        // revoke a previously injected startup token.
                        String personalSession = NativeBridge.session();
                        if (personalSession == null) throw new Exception("personal owner unavailable");
                        String html = new String(readBounded(stream, 1024 * 1024), StandardCharsets.UTF_8);
                        String script = "<script>window.__CYLEDGER_PERSONAL__=true;"
                            + "(()=>{const s=JSON.parse(" + JSONObject.quote(personalSession) + ");"
                            + "localStorage.setItem('ebk_user_token',s.token);"
                            + "localStorage.setItem('ebk_user_info',JSON.stringify(s.user));"
                            + "if(location.hash.includes('/login')||location.hash.includes('/signup'))history.replaceState(null,'','/personal');"
                            + "})();</script>";
                        File restored=new File(getFilesDir(),"ledger/settings.json");
                        if(restored.isFile()){String settings=new String(java.nio.file.Files.readAllBytes(restored.toPath()),StandardCharsets.UTF_8);new JSONObject(settings);script+="<script>(()=>{let settings=JSON.parse("+JSONObject.quote(settings).replace("<","\\u003c").replace(">","\\u003e")+");Object.keys(localStorage).filter(k=>k==='ebk_app_settings'||k.startsWith('cy_ledger_experience_')).forEach(k=>localStorage.removeItem(k));Object.entries(settings).forEach(([k,v])=>localStorage.setItem(k,v));window.addEventListener('load',()=>CYLedgerLocal.action('settingsRestored','{}'));})();</script>";}
                        html = html.replace("<head>", "<head>" + script);
                        return new WebResourceResponse("text/html", "UTF-8", 200, "OK",
                            Collections.singletonMap("Cache-Control", "no-store"),
                            new ByteArrayInputStream(html.getBytes(StandardCharsets.UTF_8)));
                    } catch (Exception error) {
                        return new WebResourceResponse("text/plain", "UTF-8", 503, "Unavailable",
                            Collections.singletonMap("Cache-Control", "no-store"),
                            new ByteArrayInputStream("个人账本页面加载失败，请关闭后重试。".getBytes(StandardCharsets.UTF_8)));
                    }
                }
                @Override public boolean shouldOverrideUrlLoading(WebView view, WebResourceRequest request) {
                    if (isLocal(request.getUrl().toString())) return false;
                    if (!request.isForMainFrame()) return true;
                    Uri uri = request.getUrl();
                    if ("https".equals(uri.getScheme())) {
                        new AlertDialog.Builder(MainActivity.this).setMessage("用浏览器打开此链接？")
                            .setPositiveButton("打开", (dialog, which) -> {
                                try { startActivity(new Intent(Intent.ACTION_VIEW, uri)); }
                                catch (Exception error) { Toast.makeText(MainActivity.this, "没有可用的浏览器", Toast.LENGTH_SHORT).show(); }
                            }).setNegativeButton("取消", null).show();
                    }
                    return true;
                }
                @Override public void onReceivedError(WebView view, WebResourceRequest request, WebResourceError error) {
                    if (request.isForMainFrame() && isLocal(request.getUrl().toString())) {
                        showStatus("手机账本页面加载失败，请重试。", true);
                    }
                }
                @Override public void onPageFinished(WebView view, String url) {
                    if (isLocal(url)) root.requestApplyInsets();
                }
            });
            web.setWebChromeClient(new WebChromeClient() {
                @Override public void onGeolocationPermissionsShowPrompt(String pageOrigin,android.webkit.GeolocationPermissions.Callback callback){if(!isLocal(pageOrigin)){callback.invoke(pageOrigin,false,false);return;}geoOrigin=pageOrigin;geoCallback=callback;if(checkSelfPermission(android.Manifest.permission.ACCESS_FINE_LOCATION)==android.content.pm.PackageManager.PERMISSION_GRANTED||checkSelfPermission(android.Manifest.permission.ACCESS_COARSE_LOCATION)==android.content.pm.PackageManager.PERMISSION_GRANTED){callback.invoke(pageOrigin,true,false);geoCallback=null;}else requestPermissions(new String[]{android.Manifest.permission.ACCESS_FINE_LOCATION,android.Manifest.permission.ACCESS_COARSE_LOCATION},804);}

                @Override public boolean onShowFileChooser(WebView view, ValueCallback<Uri[]> callback, FileChooserParams parameters) {
                    if (uploadCallback != null) uploadCallback.onReceiveValue(null);
                    uploadCallback = callback;
                    Intent intent = new Intent(Intent.ACTION_OPEN_DOCUMENT);
                    intent.addCategory(Intent.CATEGORY_OPENABLE);
                    String[] types = parameters.getAcceptTypes();
                    for(int i=0;i<types.length;i++){if(types[i].startsWith(".")){String mime=android.webkit.MimeTypeMap.getSingleton().getMimeTypeFromExtension(types[i].substring(1));types[i]=mime==null?"*/*":mime;}}
                    intent.setType(types.length == 1 && !types[0].isEmpty() ? types[0] : "*/*");
                    if (types.length > 1) intent.putExtra(Intent.EXTRA_MIME_TYPES, types);
                    intent.putExtra(Intent.EXTRA_ALLOW_MULTIPLE, parameters.getMode() == FileChooserParams.MODE_OPEN_MULTIPLE);
                    try { startActivityForResult(intent, PICK_FILE); }
                    catch (Exception error) { callback.onReceiveValue(null); uploadCallback = null; }
                    return true;
                }
            });
            web.setDownloadListener((url, userAgent, disposition, mime, size) -> {
                if (pendingDownload != null) return;
                String filename = android.webkit.URLUtil.guessFileName(url, disposition, mime);
                if (url.startsWith("blob:" + origin + "/")) {
                    // Preserve the export filename supplied by the existing Vue page.
                    String expression = "(function(){let a=Array.from(document.querySelectorAll('a[download]')).find(a=>a.href==="
                        + JSONObject.quote(url) + ");return a?a.download:'';})()";
                    web.evaluateJavascript(expression, value -> {
                        String name = filename;
                        try {
                            Object decoded = new JSONTokener(value).nextValue();
                            if (decoded instanceof String) {
                                String suggested = ((String) decoded).trim();
                                if (!suggested.isEmpty() && suggested.length() < 180 && !suggested.contains("/") && !suggested.contains("\\")) name = suggested;
                            }
                        } catch (Exception ignored) {}
                        readBlob(url, name, mime);
                    });
                }
                else if (isLocal(url)) io.execute(() -> download(url, userAgent, filename, mime));
            });
        }
        root.addView(web, new LinearLayout.LayoutParams(-1, -1));
        if (state != null && web.restoreState(state) != null) return;
        web.loadUrl(origin + "/personal"+(getIntent().getBooleanExtra("quickEntry",false)?"#!/transaction/add?type=3":""));
    }

    boolean isLocalPage(String url) { return url!=null&&isLocal(url); }
    private boolean isLocal(String url) {
        Uri uri = Uri.parse(url);
        Uri local = Uri.parse(origin);
        return "http".equals(uri.getScheme()) && "127.0.0.1".equals(uri.getHost()) && uri.getPort() == local.getPort();
    }

    private final class AppearanceBridge {
        @android.webkit.JavascriptInterface public void setTheme(String color, boolean dark) {
            if (color == null || !color.matches("#[0-9a-fA-F]{6}")) return;
            handler.post(() -> {
                if (destroyed || web == null || !isLocal(web.getUrl())) return;
                int value = Color.parseColor(color);
                appearanceDark = dark;
                root.setBackgroundColor(value);
                getWindow().setStatusBarColor(value);
                getWindow().setNavigationBarColor(value);
                if (android.os.Build.VERSION.SDK_INT >= 30) {
                    WindowInsetsController controller=getWindow().getInsetsController();
                    int flags=WindowInsetsController.APPEARANCE_LIGHT_STATUS_BARS|WindowInsetsController.APPEARANCE_LIGHT_NAVIGATION_BARS;
                    if(controller!=null)controller.setSystemBarsAppearance(dark?0:flags,flags);
                }
            });
        }
    }

    private void prepareAssets(File rootDirectory) throws Exception {
        String revision;
        try (InputStream stream = getAssets().open("revision.txt")) {
            revision = new String(readBounded(stream, 1024), StandardCharsets.UTF_8);
        }
        File marker = new File(rootDirectory, "asset-revision");
        if (marker.isFile()) {
            try (InputStream stream = new FileInputStream(marker)) {
                if (revision.equals(new String(readBounded(stream, 1024), StandardCharsets.UTF_8))) return;
            }
        }
        File publicDirectory = new File(rootDirectory, "public");
        if (!publicDirectory.isDirectory() && !publicDirectory.mkdirs()) throw new Exception("assets directory");
        String prefix = publicDirectory.getCanonicalPath() + File.separator;
        try (ZipInputStream zip = new ZipInputStream(getAssets().open("frontend.zip"))) {
            ZipEntry entry;
            while ((entry = zip.getNextEntry()) != null) {
                File target = new File(publicDirectory, entry.getName());
                if (!target.getCanonicalPath().startsWith(prefix)) throw new Exception("invalid bundled asset");
                if (entry.isDirectory()) {
                    if (!target.isDirectory() && !target.mkdirs()) throw new Exception("asset directory");
                } else {
                    File parent = target.getParentFile();
                    if (!parent.isDirectory() && !parent.mkdirs()) throw new Exception("asset parent");
                    try (OutputStream output = new FileOutputStream(target)) { copy(zip, output); }
                }
                zip.closeEntry();
            }
        }
        try (InputStream stream = getAssets().open("defaults.ini"); OutputStream output = new FileOutputStream(new File(rootDirectory, "defaults.ini"))) {
            copy(stream, output);
        }
        try (OutputStream output = new FileOutputStream(marker)) { output.write(revision.getBytes(StandardCharsets.UTF_8)); }
    }

    private void prepareTrustStore(File directory) throws Exception {
        TrustManagerFactory factory = TrustManagerFactory.getInstance(TrustManagerFactory.getDefaultAlgorithm());
        factory.init((KeyStore) null);
        try (OutputStream output = new FileOutputStream(new File(directory, "ca-certificates.pem"))) {
            for (TrustManager manager : factory.getTrustManagers()) {
                if (!(manager instanceof X509TrustManager)) continue;
                for (X509Certificate certificate : ((X509TrustManager) manager).getAcceptedIssuers()) {
                    String pem = "-----BEGIN CERTIFICATE-----\n" + Base64.encodeToString(certificate.getEncoded(), Base64.NO_WRAP).replaceAll("(.{64})", "$1\n") + "\n-----END CERTIFICATE-----\n";
                    output.write(pem.getBytes(StandardCharsets.US_ASCII));
                }
            }
        }
    }

    private static void copy(InputStream input, OutputStream output) throws Exception {
        byte[] buffer = new byte[32768];
        int count;
        while ((count = input.read(buffer)) != -1) output.write(buffer, 0, count);
    }

    private static byte[] readBounded(InputStream input, int limit) throws Exception {
        ByteArrayOutputStream output = new ByteArrayOutputStream();
        byte[] buffer = new byte[32768];
        int count;
        while ((count = input.read(buffer)) != -1) {
            if (output.size() + count > limit) throw new Exception("file too large");
            output.write(buffer, 0, count);
        }
        return output.toByteArray();
    }

    private void download(String url, String userAgent, String name, String mime) {
        HttpURLConnection connection = null;
        try {
            connection = (HttpURLConnection) new URL(url).openConnection();
            connection.setInstanceFollowRedirects(false);
            connection.setConnectTimeout(10000);
            connection.setReadTimeout(60000);
            connection.setRequestProperty("User-Agent", userAgent);
            String cookies = CookieManager.getInstance().getCookie(url);
            if (cookies != null) connection.setRequestProperty("Cookie", cookies);
            if (connection.getResponseCode() != 200) throw new Exception("download failed");
            byte[] data;
            try (InputStream stream = connection.getInputStream()) { data = readBounded(stream, MAX_DOWNLOAD); }
            handler.post(() -> saveDownload(data, name, mime));
        } catch (Exception error) {
            handler.post(() -> Toast.makeText(this, "导出失败，文件上限为 20 MB", Toast.LENGTH_LONG).show());
        } finally { if (connection != null) connection.disconnect(); }
    }

    private void readBlob(String url, String name, String mime) {
        String script = "window.__cyledgerDownload=null;fetch(" + JSONObject.quote(url) + ")"
            + ".then(r=>r.blob()).then(b=>{if(b.size>" + MAX_DOWNLOAD + ")throw Error('large');"
            + "let r=new FileReader();r.onload=()=>window.__cyledgerDownload={data:r.result.split(',')[1],mime:b.type};"
            + "r.onerror=()=>window.__cyledgerDownload={error:true};r.readAsDataURL(b)})"
            + ".catch(()=>window.__cyledgerDownload={error:true});";
        web.evaluateJavascript(script, result -> pollBlob(name, mime, 0));
    }

    private void pollBlob(String name, String mime, int attempt) {
        if (destroyed || attempt >= 300) return;
        web.evaluateJavascript("JSON.stringify(window.__cyledgerDownload)", value -> {
            try {
                Object raw = new JSONTokener(value).nextValue();
                if (!(raw instanceof String) || "null".equals(raw)) {
                    handler.postDelayed(() -> pollBlob(name, mime, attempt + 1), 100);
                    return;
                }
                JSONObject result = new JSONObject((String) raw);
                web.evaluateJavascript("delete window.__cyledgerDownload", null);
                if (result.optBoolean("error")) throw new Exception("blob download");
                saveDownload(Base64.decode(result.getString("data"), Base64.DEFAULT), name, result.optString("mime", mime));
            } catch (Exception error) { Toast.makeText(this, "导出失败，文件上限为 20 MB", Toast.LENGTH_LONG).show(); }
        });
    }

    private void saveDownload(byte[] data, String name, String mime) {
        if (destroyed || data.length > MAX_DOWNLOAD) return;
        pendingDownload = data;
        Intent intent = new Intent(Intent.ACTION_CREATE_DOCUMENT);
        intent.addCategory(Intent.CATEGORY_OPENABLE);
        String mimeType = mime == null || mime.isEmpty() ? "application/octet-stream" : mime.split(";", 2)[0].trim();
        intent.setType(mimeType);
        intent.putExtra(Intent.EXTRA_TITLE, name);
        try { startActivityForResult(intent, SAVE_FILE); }
        catch (Exception error) { pendingDownload = null; Toast.makeText(this, "无法打开文件保存位置", Toast.LENGTH_LONG).show(); }
    }

    @Override protected void onActivityResult(int requestCode, int resultCode, Intent result) {
        super.onActivityResult(requestCode, resultCode, result);
        local.activityResult(requestCode,resultCode,result);
        if (requestCode == PICK_FILE && uploadCallback != null) {
            Uri[] files=null; if(resultCode==RESULT_OK && result!=null){android.content.ClipData clip=result.getClipData();if(clip!=null){files=new Uri[clip.getItemCount()];for(int i=0;i<files.length;i++)files[i]=clip.getItemAt(i).getUri();}else if(result.getData()!=null)files=new Uri[]{result.getData()};}
            local.selectedFiles(files);
            uploadCallback.onReceiveValue(files);
            uploadCallback = null;
        }
        if (requestCode == SAVE_FILE && pendingDownload != null) {
            byte[] data = pendingDownload;
            pendingDownload = null;
            if (resultCode == RESULT_OK && result != null && result.getData() != null) {
                Uri uri = result.getData();
                io.execute(() -> {
                    try (OutputStream output = getContentResolver().openOutputStream(uri)) {
                        if (output == null) throw new Exception("output unavailable");
                        output.write(data);
                        handler.post(() -> Toast.makeText(this, "文件已保存", Toast.LENGTH_SHORT).show());
                    } catch (Exception error) { handler.post(() -> Toast.makeText(this, "文件保存失败", Toast.LENGTH_LONG).show()); }
                });
            }
        }
    }

    @Override public void onBackPressed() {
        if (loaded && web != null) {
            web.evaluateJavascript("(function(){let modal=document.querySelector('.modal-in');if(modal){let close=modal.querySelector('.popup-close,.sheet-close,.dialog-button');if(close){close.click();return true;}let api=modal.f7Modal;if(api){api.close();return true;}}let v=document.querySelector('.view-main');if(v&&v.f7View&&v.f7View.router.history.length>1){v.f7View.router.back();return true;}return false;})()", result -> {
                if (!"true".equals(result)) {
                    if (web.canGoBack()) web.goBack();
                    else exitLedger();
                }
            });
        } else exitLedger();
    }

    private void exitLedger(){if(local.exitConfirm())new AlertDialog.Builder(this).setTitle("退出账本？").setNegativeButton("取消",null).setPositiveButton("退出",(d,w)->moveTaskToBack(true)).show();else moveTaskToBack(true);}
    @Override protected void onStop(){local.stopped();super.onStop();}
    @Override protected void onResume(){super.onResume();if(local!=null)local.resumed();}
    @Override protected void onNewIntent(Intent intent){super.onNewIntent(intent);setIntent(intent);if(intent.getBooleanExtra("quickEntry",false)&&web!=null)local.ensureUnlocked(()->web.evaluateJavascript("document.querySelector('.view-main')?.f7View?.router.navigate('/transaction/add?type=3')",null));}
    @Override public void onRequestPermissionsResult(int request,String[] permissions,int[] results){super.onRequestPermissionsResult(request,permissions,results);if(request==804&&geoCallback!=null){boolean granted=false;for(int result:results)if(result==android.content.pm.PackageManager.PERMISSION_GRANTED)granted=true;geoCallback.invoke(geoOrigin,granted,false);geoCallback=null;}}
    @Override public void onConfigurationChanged(Configuration configuration) {
        super.onConfigurationChanged(configuration);
        applyInsets();
    }

    @Override protected void onSaveInstanceState(Bundle state) {
        if (web != null) web.saveState(state);
        super.onSaveInstanceState(state);
    }

    @Override protected void onDestroy() {
        destroyed = true;
        if (android.os.Build.VERSION.SDK_INT >= 33 && backCallback != null) {
            getOnBackInvokedDispatcher().unregisterOnBackInvokedCallback(backCallback);
        }
        handler.removeCallbacksAndMessages(null);
        if (uploadCallback != null) uploadCallback.onReceiveValue(null);
        if (web != null) { root.removeView(web); web.destroy(); }
        io.shutdownNow();
        if(local!=null)local.destroy();
        super.onDestroy();
    }
}
