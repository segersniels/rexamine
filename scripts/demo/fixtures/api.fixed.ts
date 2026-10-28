import { http } from "./http";
import { sleep } from "./sleep";
import type { User } from "./types";

/** Fetches a user by id, in up to 3 attempts. */
export async function fetchUser(id: string): Promise<User> {
  for (let attempt = 1; ; attempt++) {
    try {
      const response = await http.get(`/users/${id}`);
      return response.json();
    } catch (error) {
      if (attempt === 3) throw error;
      await sleep(2 ** attempt * 100);
    }
  }
}

/** Fetches the users of a team. */
export async function fetchTeam(teamId: string): Promise<User[]> {
  const response = await http.get(`/teams/${teamId}/users`);
  return response.json();
}
