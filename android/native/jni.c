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

JNIEXPORT jstring JNICALL Java_com_cyledger_android_NativeBridge_backup(JNIEnv *env,jclass type,jstring directory,jstring destination,jstring settings){
const char *a=(*env)->GetStringUTFChars(env,directory,NULL),*b=(*env)->GetStringUTFChars(env,destination,NULL),*c=(*env)->GetStringUTFChars(env,settings,NULL);
char *value=CYLedgerBackup((char*)a,(char*)b,(char*)c);(*env)->ReleaseStringUTFChars(env,directory,a);(*env)->ReleaseStringUTFChars(env,destination,b);(*env)->ReleaseStringUTFChars(env,settings,c);jstring result=(*env)->NewStringUTF(env,value);CYLedgerFree(value);return result;}
JNIEXPORT jstring JNICALL Java_com_cyledger_android_NativeBridge_stageRestore(JNIEnv *env,jclass type,jstring archive,jstring parent){
const char *a=(*env)->GetStringUTFChars(env,archive,NULL),*b=(*env)->GetStringUTFChars(env,parent,NULL);char *value=CYLedgerStageRestore((char*)a,(char*)b);(*env)->ReleaseStringUTFChars(env,archive,a);(*env)->ReleaseStringUTFChars(env,parent,b);jstring result=(*env)->NewStringUTF(env,value);CYLedgerFree(value);return result;}
