plugins {
    alias(libs.plugins.android.application)
    id("couchverse.android.compose")
    alias(libs.plugins.kotlin.serialization)
}

android {
    namespace = "io.stepes.couchverse"
    compileSdk = 37

    defaultConfig {
        // builders who sign their own APKs can pick another id (D24): -Pcouchverse.applicationId=...
        applicationId = providers.gradleProperty("couchverse.applicationId")
            .getOrElse("io.stepes.couchverse")
        minSdk = 31
        targetSdk = 37
        versionCode = providers.gradleProperty("couchverse.versionCode").map(String::toInt).getOrElse(1)
        versionName = providers.gradleProperty("couchverse.version").getOrElse("dev")
    }

    // A release APK is signed with the keystore these name (CI passes its repository secrets);
    // without one it falls back to the debug key, so a build from source still installs.
    val keystore = providers.environmentVariable("COUCHVERSE_KEYSTORE").orNull
    signingConfigs {
        if (keystore != null) {
            create("release") {
                storeFile = file(keystore)
                storePassword = providers.environmentVariable("COUCHVERSE_KEYSTORE_PASSWORD").get()
                keyAlias = providers.environmentVariable("COUCHVERSE_KEY_ALIAS").get()
                keyPassword = providers.environmentVariable("COUCHVERSE_KEY_PASSWORD").get()
            }
        }
    }

    buildTypes {
        release {
            isMinifyEnabled = true
            isShrinkResources = true
            proguardFiles(getDefaultProguardFile("proguard-android-optimize.txt"), "proguard-rules.pro")
            signingConfig = signingConfigs.findByName("release") ?: signingConfigs.getByName("debug")
        }
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    buildFeatures {
        buildConfig = true
    }

    testOptions {
        unitTests.isIncludeAndroidResources = true
    }

    lint {
        lintConfig = rootProject.file("lint.xml")
        checkDependencies = true
    }
}

tasks.withType<Test>().configureEach {
    // Android 16's shared memory setup reaches into file descriptors through JDK internals
    jvmArgs("--add-exports=java.base/jdk.internal.access=ALL-UNNAMED")
}

dependencies {
    implementation(project(":core"))
    implementation(project(":design"))
    implementation(project(":feature-accounts"))
    implementation(project(":feature-catalog"))
    implementation(project(":feature-couch"))
    implementation(project(":feature-downloads"))
    implementation(project(":feature-playback"))
    implementation(project(":feature-ranks"))
    implementation(project(":feature-settings"))

    implementation(libs.androidx.core.ktx)
    implementation(libs.androidx.activity.compose)
    implementation(libs.androidx.lifecycle.runtime.compose)
    implementation(libs.androidx.lifecycle.process)
    implementation(libs.androidx.navigation.compose)
    implementation(libs.coil.network.okhttp)
    implementation(libs.androidx.work.runtime)
    implementation(libs.androidx.glance.appwidget)
    implementation(libs.androidx.tvprovider)

    testImplementation(project(":testing"))
    testImplementation(libs.kotlin.test.junit)
    debugImplementation(libs.androidx.compose.ui.test.manifest)
}
