package com.cyledger.android;

final class NativeBridge {
    static {
        System.loadLibrary("cyledger_backend");
        System.loadLibrary("cyledger_jni");
    }

    // Blocks on a dedicated Java thread; the service shares the app's lifetime.
    static native String start(String directory, int port);
    static native String session();
    static native void directNetwork(long handle, String dnsServers);
    static native String backup(String directory,String destination,String settings);
    static native String stageRestore(String archive,String parent);

    private NativeBridge() {}
}
