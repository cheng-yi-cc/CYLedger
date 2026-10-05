#include <jni.h>
#include "libcyledger_backend.h"

JNIEXPORT jstring JNICALL
Java_com_cyledger_android_NativeBridge_start(JNIEnv *env, jclass type, jstring directory, jint port) {
    const char *root = (*env)->GetStringUTFChars(env, directory, NULL);
    if (root == NULL) return NULL;
    char *error = CYLedgerStart((char *)root, port);
    (*env)->ReleaseStringUTFChars(env, directory, root);
    if (error == NULL) return NULL;
    jstring result = (*env)->NewStringUTF(env, error);
    CYLedgerFree(error);
    return result;
}

JNIEXPORT jstring JNICALL
Java_com_cyledger_android_NativeBridge_session(JNIEnv *env, jclass type) {
    char *session = CYLedgerSession();
    if (session == NULL) return NULL;
    jstring result = (*env)->NewStringUTF(env, session);
    CYLedgerFree(session);
    return result;
}
