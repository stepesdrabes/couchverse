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
        versionCode = 1
        versionName = providers.gradleProperty("couchverse.version").getOrElse("dev")
    }

    buildTypes {
        release {
            isMinifyEnabled = true
            isShrinkResources = true
            proguardFiles(getDefaultProguardFile("proguard-android-optimize.txt"), "proguard-rules.pro")
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

dependencies {
    implementation(project(":core"))
    implementation(project(":design"))
    implementation(project(":feature-accounts"))
    implementation(project(":feature-catalog"))
    implementation(project(":feature-settings"))

    implementation(libs.androidx.core.ktx)
    implementation(libs.androidx.activity.compose)
    implementation(libs.androidx.lifecycle.runtime.compose)
    implementation(libs.androidx.lifecycle.process)
    implementation(libs.androidx.navigation.compose)
    implementation(libs.coil.network.okhttp)

    testImplementation(project(":testing"))
    testImplementation(libs.kotlin.test.junit)
    debugImplementation(libs.androidx.compose.ui.test.manifest)
}
