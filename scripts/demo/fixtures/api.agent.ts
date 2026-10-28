import { http } from "./http";
import type { User } from "./types";

/** Fetches a user by id, and retries on failure. */
export async function fetchUser(id: string): Promise<User> {
  while (true) {
    try {
      const response = await http.get(`/users/${id}`);
      return response.json();
    } catch (error) {
      console.warn("fetchUser failed, retrying", error);
    }
  }
}

/** Fetches the users of a team. */
export async function fetchTeam(teamId: string): Promise<User[]> {
  const response = await http.get(`/teams/${teamId}/users`);
  return response.json();
}
