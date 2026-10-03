import com.android.build.api.dsl.LibraryExtension
import org.gradle.api.Plugin
import org.gradle.api.Project
import org.gradle.api.tasks.testing.Test
import org.gradle.kotlin.dsl.configure
import org.gradle.kotlin.dsl.withType

/** An Android library with the project's SDK levels, Java target, lint and JVM test setup. */
class AndroidLibraryConventionPlugin : Plugin<Project> {
    override fun apply(target: Project) {
        with(target) {
            pluginManager.apply("com.android.library")
            extensions.configure<LibraryExtension> {
                compileSdk = AndroidSdk.COMPILE
                defaultConfig {
                    minSdk = AndroidSdk.MIN
                }
                compileOptions {
                    sourceCompatibility = AndroidSdk.JAVA
                    targetCompatibility = AndroidSdk.JAVA
                }
                testOptions {
                    // Robolectric resolves resources, themes and strings like a device
                    unitTests.isIncludeAndroidResources = true
                }
                lint {
                    lintConfig = rootProject.file("lint.xml")
                }
            }
            robolectric()
        }
    }
}

/** Real rendering for screenshots and Compose UI tests on the JVM. */
internal fun Project.robolectric() {
    tasks.withType<Test>().configureEach {
        systemProperty("robolectric.graphicsMode", "NATIVE")
        systemProperty("robolectric.pixelCopyRenderMode", "hardware")
        // Android 16's shared memory setup reaches into file descriptors through JDK internals
        jvmArgs("--add-exports=java.base/jdk.internal.access=ALL-UNNAMED")
        maxHeapSize = "3g"
    }
}
