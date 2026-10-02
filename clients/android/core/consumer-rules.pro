# JNA binds the native functions and the UniFFI structures by reflection
-keep class com.sun.jna.** { *; }
-keep class * implements com.sun.jna.** { *; }
-keep class io.stepes.couchverse.core.ffi.** { *; }
-dontwarn java.awt.**
