import { api } from '$lib/api/client';
import type { Genre } from './types';

export const listGenres = () => api<Genre[]>('/genres');
