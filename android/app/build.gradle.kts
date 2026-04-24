plugins {
	id("com.android.application") version "8.8.2"
	id("org.jetbrains.kotlin.android") version "2.0.21"
}

android {
	namespace = "io.github.asciimoth.gonnectvpnexample"
	compileSdk = 35

	defaultConfig {
		applicationId = "io.github.asciimoth.gonnectvpnexample"
		minSdk = 21
		targetSdk = 35
		versionCode = 1
		versionName = "0.1.0"
	}

	buildTypes {
		debug {
			isMinifyEnabled = false
		}
		release {
			isMinifyEnabled = false
			proguardFiles(
				getDefaultProguardFile("proguard-android-optimize.txt"),
				"proguard-rules.pro",
			)
		}
	}

	compileOptions {
		sourceCompatibility = JavaVersion.VERSION_17
		targetCompatibility = JavaVersion.VERSION_17
	}

	kotlinOptions {
		jvmTarget = "17"
	}

	buildFeatures {
		viewBinding = true
	}
}

dependencies {
	implementation(files("${rootDir.parentFile}/build/android/mobilelib.aar"))
	implementation("androidx.core:core-ktx:1.15.0")
	implementation("androidx.appcompat:appcompat:1.7.1")
	implementation("androidx.lifecycle:lifecycle-runtime-ktx:2.8.7")
	implementation("androidx.lifecycle:lifecycle-viewmodel-ktx:2.8.7")
	implementation("org.jetbrains.kotlinx:kotlinx-coroutines-android:1.10.2")
}

tasks.named("preBuild").configure {
	dependsOn(":generateGoMobileAar")
}
