package fr.ialocale.mobile;
import java.io.*;
import java.nio.file.*;
import java.util.*;
public class ModelFilesTest {
 static void check(boolean b){if(!b)throw new AssertionError();}
 static void rejected(byte[] bytes,File d,boolean cancel) throws Exception {int before=d.list().length;try{ModelFiles.copy(new ByteArrayInputStream(bytes),d,()->cancel);throw new AssertionError("acceptation erronee");}catch(IOException expected){}check(d.list().length==before);}
 public static void main(String[] a)throws Exception{File d=Files.createTempDirectory("gguf-test").toFile();byte[] valid=new byte[64];System.arraycopy(new byte[]{'G','G','U','F'},0,valid,0,4);File f=ModelFiles.copy(new ByteArrayInputStream(valid),d,()->false);check(Arrays.equals(valid,Files.readAllBytes(f.toPath())));System.out.println("OK copie exacte");rejected(new byte[64],d,false);System.out.println("OK refus non GGUF");rejected(Arrays.copyOf(valid,8),d,false);System.out.println("OK refus fichier trop court");rejected(valid,d,true);System.out.println("OK annulation sans reste");File g=ModelFiles.copy(new ByteArrayInputStream(valid),d,()->false);check(f.equals(g)&&d.list().length==1);System.out.println("OK doublon évité");}
}
