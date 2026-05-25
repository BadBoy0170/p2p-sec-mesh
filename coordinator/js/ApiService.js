export class ApiService {
    constructor(endpoint) {
        this.endpoint = endpoint;
    }

    async getTopology() {
        try {
            const response = await fetch(this.endpoint);
            return await response.json();
        } catch (error) {
            console.error("API Fetch Error:", error);
            throw error;
        }
    }
}
