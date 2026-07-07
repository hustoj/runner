public class Main {
	public static void main(String[] args) {
		byte[] chunk = new byte[65536];
		for (int i = 0; i < chunk.length; i++) {
			chunk[i] = 'A';
		}

		for (int i = 0; i < 64; i++) {
			System.out.write(chunk, 0, chunk.length);
		}
		System.out.flush();
	}
}
