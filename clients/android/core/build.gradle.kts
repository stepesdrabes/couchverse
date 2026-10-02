plugins {
    alias(libs.plugins.android.library)
    alias(libs.plugins.kotlin.serialization)
}

// `make core-android` writes the native libraries to src/main/jniLibs, and the UniFFI bindings
// and the host library for JVM tests under build/ (so a `clean` removes those two)
val uniffiBindings = layout.buildDirectory.dir("generated/uniffi")
val hostLibrary = layout.buildDirectory.dir("rust-host")

android {
    namespace = "io.stepes.couchverse.core"
    compileSdk = 37
    ndkVersion = libs.versions.ndk.get()

    defaultConfig {
        minSdk = 31
        consumerProguardFiles("consumer-rules.pro")
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    sourceSets {
        named("main") {
            // the message types (`make contract`) and the UniFFI bindings (`make core-android`)
            kotlin.directories += "src/generated/kotlin"
            kotlin.directories += uniffiBindings.get().asFile.path
        }
    }
}

dependencies {
    // the generated message types are the module's API and carry their serializers
    api(libs.kotlinx.serialization.json)
    implementation(libs.jna) { artifact { type = "aar" } }

    // the aar has no desktop natives; the jar brings JNA's dispatch library for the host JVM
    testImplementation(libs.jna)
    testImplementation(libs.kotlin.test.junit)
}

val checkRustCore = tasks.register("checkRustCore") {
    description = "Fails early, with the fix, when the outputs of `make core-android` are missing."
    val required = listOf("arm64-v8a", "armeabi-v7a", "x86_64").map {
        file("src/main/jniLibs/$it/libcouchverse_ffi.so")
    } + uniffiBindings.get().file("io/stepes/couchverse/core/ffi/couchverse_ffi.kt").asFile +
        hostLibrary.get().file(System.mapLibraryName("couchverse_ffi")).asFile
    doLast {
        val missing = required.filterNot { it.exists() }
        if (missing.isNotEmpty()) {
            throw GradleException(
                "The Rust core is not built for Android; run `make core-android` from the " +
                    "repository root. Missing:\n" + missing.joinToString("\n") { "  $it" },
            )
        }
    }
}

tasks.named("preBuild") { dependsOn(checkRustCore) }

tasks.withType<Test>().configureEach {
    systemProperty("jna.library.path", hostLibrary.get().asFile.path)
}
