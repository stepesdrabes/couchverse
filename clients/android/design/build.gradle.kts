plugins {
    alias(libs.plugins.android.library)
}

android {
    namespace = "io.stepes.couchverse.design"
    compileSdk = 37

    defaultConfig {
        minSdk = 31
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    sourceSets {
        named("main") {
            // the tokens and string catalogs generated from contract/ by `make contract`
            kotlin.directories += "src/generated/kotlin"
            res.directories += "src/generated/res"
        }
    }
}

dependencies {
    // the tokens are Compose colours and dimensions
    api(platform(libs.androidx.compose.bom))
    api(libs.androidx.compose.ui.graphics)
    api(libs.androidx.compose.ui.unit)
}
