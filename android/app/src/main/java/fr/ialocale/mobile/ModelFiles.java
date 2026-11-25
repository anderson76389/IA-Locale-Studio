package fr.ialocale.mobile;
import java.io.*;
import java.nio.file.*;
import java.util.UUID;
import java.util.function.BooleanSupplier;
final class ModelFiles {
    static File copy(InputStream input, File dir, BooleanSupplier cancelled) throws IOException {
        try (InputStream in = input) {
            if (!dir.isDirectory() && !dir.mkdirs()) throw new IOException("Dossier inaccessible");
            File part = File.createTempFile("model-", ".part", dir);
            File dest = new File(dir, "modele-" + UUID.randomUUID() + ".gguf");
            try {
                try (FileOutputStream out = new FileOutputStream(part)) {
                    byte[] magic = new byte[4]; int p=0;
                    while(p<4){int n=in.read(magic,p,4-p);if(n<0) throw new IOException("Fichier incomplet");p+=n;}
                    if(magic[0]!='G'||magic[1]!='G'||magic[2]!='U'||magic[3]!='F') throw new IOException("Fichier GGUF requis");
                    out.write(magic); byte[] buf=new byte[128*1024]; int n; long total=4;
                    while((n=in.read(buf))!=-1){if(cancelled.getAsBoolean()) throw new IOException("Import annule");out.write(buf,0,n);total+=n;}
                    if(total<24 || cancelled.getAsBoolean()) throw new IOException("Import incomplet ou annule");
                    out.getFD().sync();
                }
                Files.move(part.toPath(),dest.toPath()); return dest;
            } finally {part.delete();}
        }
    }
}
