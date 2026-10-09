import {
	Globe,
	Monitor,
	MonitorSmartphone,
	Smartphone,
	Tablet,
	Tv,
	type Icon
} from 'lucide-svelte';
import * as m from '$lib/paraglide/messages';
import type { DevicePlatform } from './api';

const PLATFORMS: Record<DevicePlatform, { icon: typeof Icon; name: () => string }> = {
	ios: { icon: Smartphone, name: m.devices_platform_ios },
	ipados: { icon: Tablet, name: m.devices_platform_ipados },
	tvos: { icon: Tv, name: m.devices_platform_tvos },
	android: { icon: Smartphone, name: m.devices_platform_android },
	androidtv: { icon: Tv, name: m.devices_platform_androidtv },
	desktop: { icon: Monitor, name: m.devices_platform_desktop },
	web: { icon: Globe, name: m.devices_platform_web }
};

// A platform the server learns before this build of the web does still gets an
// icon and a readable name.
export const platformIcon = (platform: string): typeof Icon =>
	PLATFORMS[platform as DevicePlatform]?.icon ?? MonitorSmartphone;

export const platformName = (platform: string): string =>
	PLATFORMS[platform as DevicePlatform]?.name() ?? platform;
