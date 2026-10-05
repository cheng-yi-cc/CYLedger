package com.cyledger.android;

final class NativeBridge {
    static {
        System.loadLibrary("cyledger_backend");
        System.loadLibrary("cyledger_jni");
    }

    // Blocks on a dedicated Java thread; the service shares the app's lifetime.
    static native String start(String directory, int port);
    static native String session();

    private NativeBridge() {}
}
