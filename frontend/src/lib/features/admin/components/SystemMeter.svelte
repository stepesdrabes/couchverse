<script lang="ts">
	import { Cpu, MemoryStick } from 'lucide-svelte';
	import * as jobsApi from '$lib/features/jobs/api';
	import type { SystemStats } from '$lib/features/jobs/api';
	import { usageColor } from '$lib/utils/usage-color';

	let system = $state<SystemStats | null>(null);

	$effect(() => {
		async function poll() {
			if (document.visibilityState === 'hidden') return;
			try {
				system = await jobsApi.getSystem();
			} catch {
				// transient
			}
		}
		poll();
		const t = setInterval(poll, 5000);
		return () => clearInterval(t);
	});

	const memPercent = $derived(
		system && system.memTotal > 0 ? (system.memUsed / system.memTotal) * 100 : -1
	);
</script>

{#if system && (system.cpuPercent >= 0 || memPercent >= 0)}
	<div class="space-y-2 rounded-card border border-edge bg-surface p-3">
		{#if system.cpuPercent >= 0}
			<div>
				<div class="mb-1 flex items-center justify-between text-[11px]">
					<span class="flex items-center gap-1.5 text-muted"><Cpu class="size-3" /> CPU</span>
					<span class="text-faint tnum">{system.cpuPercent.toFixed(0)}%</span>
				</div>
				<div class="h-1 overflow-hidden rounded-full bg-surface-2">
					<div
						class="h-full rounded-full transition-colors duration-500"
						style="width: {system.cpuPercent}%; background: {usageColor(system.cpuPercent)}"
					></div>
				</div>
			</div>
		{/if}
		{#if memPercent >= 0}
			<div>
				<div class="mb-1 flex items-center justify-between text-[11px]">
					<span class="flex items-center gap-1.5 text-muted">
						<MemoryStick class="size-3" /> RAM
					</span>
					<span class="text-faint tnum">{memPercent.toFixed(0)}%</span>
				</div>
				<div class="h-1 overflow-hidden rounded-full bg-surface-2">
					<div
						class="h-full rounded-full transition-colors duration-500"
						style="width: {memPercent}%; background: {usageColor(memPercent)}"
					></div>
				</div>
			</div>
		{/if}
	</div>
{/if}
