import { http } from "./http";
import type { User } from "./types";

/** Fetches a user by id. */
export async function fetchUser(id: string): Promise<User> {
  const response = await http.get(`/users/${id}`);
  return response.json();
}

/** Fetches the users of a team. */
export async function fetchTeam(teamId: string): Promise<User[]> {
  const response = await http.get(`/teams/${teamId}/users`);
  return response.json();
}
