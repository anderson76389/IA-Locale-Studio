package fr.ialocale.mobile;
import android.app.Activity;
import android.os.Bundle;
import android.content.Intent;
import android.content.Context;
import android.graphics.Color;
import android.content.res.ColorStateList;
import android.graphics.drawable.Drawable;
import android.graphics.drawable.GradientDrawable;
import android.graphics.drawable.RippleDrawable;
import android.graphics.drawable.StateListDrawable;
import android.net.*;
import android.view.View;
import android.widget.*;
import java.io.*;
import java.nio.charset.StandardCharsets;
import java.util.concurrent.*;
import java.util.concurrent.atomic.AtomicBoolean;
public final class MainActivity extends Activity {
    private static final int PICK=40;
    private final ExecutorService worker=Executors.newSingleThreadExecutor();
    private final AtomicBoolean cancelled=new AtomicBoolean();
    private boolean nativeReady=false,busy=false, code=false;
    private TextView state, modelLabel, output, connection;
    private EditText prompt;
    private Button run, stop, pick, download, textTab, codeTab;
    private File model;
    private Conversations conversations;
    private String thread;
    private String category(){return code?"Code":"Texte";}
    private String display(File f){return getPreferences(0).getString("name-"+f.getName(),f.getName());}
    private void models(){
        if(busy)return;
        File[] files=new File(getFilesDir(),"models").listFiles((d,n)->n.endsWith(".gguf"));
        if(files==null||files.length==0){state.setText("Aucun modèle installé.");return;}
        String[] names=new String[files.length];
        for(int i=0;i<files.length;i++)names[i]=display(files[i])+" · "+files[i].length()/1048576+" Mio";
        new android.app.AlertDialog.Builder(this).setTitle("Modèles · "+getFilesDir().getUsableSpace()/1048576+" Mio libres")
            .setItems(names,(dialog,index)->{
                File f=files[index];
                new android.app.AlertDialog.Builder(this).setTitle(names[index])
                .setPositiveButton("Utiliser",(a,b)->{model=f;getPreferences(0).edit().putString("model",f.getName()).apply();refresh();})
                .setNegativeButton("Supprimer définitivement",(a,b)->{
                    if(!f.delete()){state.setText("Suppression impossible.");return;}
                    if(f.equals(model)){model=null;getPreferences(0).edit().remove("model").apply();}
                    getPreferences(0).edit().remove("name-"+f.getName()).apply();refresh();
                }).setNeutralButton("Annuler",null).show();
            }).show();
    }
    private void history(){
        if(busy)return;
        java.util.ArrayList<String[]> rows=conversations.list(category());
        if(rows.isEmpty()){state.setText("Aucune discussion dans cet atelier.");return;}
        String[] titles=new String[rows.size()];
        for(int i=0;i<titles.length;i++){String title=rows.get(i)[1];titles[i]=title.substring(0,Math.min(70,title.length()));}
        new android.app.AlertDialog.Builder(this).setTitle("Historique · "+category()).setItems(titles,(a,i)->{
            String selected=rows.get(i)[0];
            new android.app.AlertDialog.Builder(this).setTitle(titles[i])
                .setPositiveButton("Reprendre",(x,y)->{thread=selected;output.setText(conversations.read(thread));prompt.setText("");})
                .setNegativeButton("Supprimer définitivement",(x,y)->{conversations.remove(selected);if(selected.equals(thread)){thread=null;output.setText("");prompt.setText("");}})
                .setNeutralButton("Annuler",null).show();
        }).show();
    }
    private static native byte[] generateNative(String path, byte[] input, int maxTokens, int threads);
    private static native void resetStop();
    private static native void stopNative();
    private int dp(int x){return (int)(getResources().getDisplayMetrics().density*x+.5f);}
    private TextView label(String s,int size){TextView t=new TextView(this);t.setText(s);t.setTextSize(size);t.setTextColor(Color.WHITE);t.setPadding(0,dp(8),0,dp(8));return t;}
    private Button button(String s,View.OnClickListener listener){Button b=new Button(this);b.setText(s);b.setAllCaps(false);b.setOnClickListener(listener);return b;}
    @Override public void onCreate(Bundle savedState){super.onCreate(savedState);
        try{System.loadLibrary("ialocale");nativeReady=true;}catch(LinkageError e){nativeReady=false;}
        conversations=new Conversations(this);String previous=getPreferences(0).getString("model","");if(!previous.isEmpty()){File f=new File(getFilesDir(),"models/"+new File(previous).getName());if(f.isFile()) model=f;}
        ScrollView scroll=new ScrollView(this);scroll.setFillViewport(true);
        LinearLayout root=new LinearLayout(this);root.setOrientation(LinearLayout.VERTICAL);root.setPadding(dp(20),dp(32),dp(20),dp(24));root.setBackgroundColor(0xff111521);scroll.addView(root);
        root.addView(label("IA Locale Studio",28));root.addView(label("Android • Calcul local CPU",15));connection=label("",14);root.addView(connection);
        root.addView(createWorkshopBar());
        root.addView(label("Image · Vidéo · Musique : non disponibles dans cette édition native.",14));
        modelLabel=label("",14);root.addView(modelLabel);pick=button("Importer un modèle GGUF",v->{if(busy)return;Intent i=new Intent(Intent.ACTION_OPEN_DOCUMENT);i.setType("*/*");i.addCategory(Intent.CATEGORY_OPENABLE);startActivityForResult(i,PICK);});root.addView(pick);
        download=button("Télécharger un modèle sur Hugging Face",v->{refreshNetwork();if(!online()){state.setText("Vous êtes hors ligne. Connectez-vous pour télécharger un modèle. Les modèles installés restent utilisables.");return;}try{startActivity(new Intent(Intent.ACTION_VIEW,Uri.parse("https://huggingface.co/models?library=gguf")));}catch(Exception e){state.setText("Aucun navigateur disponible.");}});root.addView(download);
        root.addView(button("Gérer les modèles",v->models()));
        root.addView(button("Historique",v->history()));
        root.addView(button("Nouvelle discussion",v->{if(!busy){thread=null;output.setText("");prompt.setText("");}}));
        root.addView(button("Tester hors ligne",v->{if(model==null){state.setText("Importez un modèle GGUF pour le test.");return;}start(true);}));
        root.addView(button("Joindre un document, une image ou un audio",v->{
            if(busy)return;Intent i=new Intent(Intent.ACTION_OPEN_DOCUMENT);i.setType("*/*");i.putExtra(Intent.EXTRA_MIME_TYPES,new String[]{"text/*","application/json","image/*","audio/*","video/*"});startActivityForResult(i,41);
        }));
        root.addView(button("Modèle de transcription Vosk",v->{
            if(busy)return;
            File vocal=Attachments.speechModel(this);
            if(vocal.isDirectory()){
                new android.app.AlertDialog.Builder(this).setMessage("Supprimer le modèle vocal installé ?")
                .setPositiveButton("Supprimer",(a,b)->state.setText(Attachments.erase(vocal)?"Modèle vocal supprimé.":"Suppression incomplète.")).setNegativeButton("Annuler",null).show();
            }else{Intent i=new Intent(Intent.ACTION_OPEN_DOCUMENT);i.setType("*/*");startActivityForResult(i,42);}
        }));
        root.addView(label("Demande",16));prompt=new EditText(this);prompt.setHint("Écrire une demande…");prompt.setTextColor(Color.WHITE);prompt.setHintTextColor(0xffa5aabb);prompt.setMinLines(4);root.addView(prompt);
        run=button("Générer localement",v->start(false));root.addView(run);stop=button("Arrêter",v->{cancelled.set(true);if(nativeReady)stopNative();state.setText("Arrêt demandé…");});root.addView(stop);state=label("Prêt",14);root.addView(state);output=label("",16);output.setTextIsSelectable(true);root.addView(output);setContentView(scroll);select(false);refresh();refreshNetwork();
    }
    private LinearLayout createWorkshopBar() {
        LinearLayout bar = new LinearLayout(this);
        bar.setOrientation(LinearLayout.HORIZONTAL);
        bar.setPadding(dp(6), dp(6), dp(6), dp(6));
        bar.setBackground(glassGradient(24, 0xff4b435f, 0xff251f35, 0xff302940));
        bar.setElevation(dp(5));
        textTab = workshopButton("Texte", false, 0xff6b588b, 0xff433457);
        codeTab = workshopButton("Code", true, 0xff356875, 0xff244451);
        bar.addView(textTab, new LinearLayout.LayoutParams(0, -2, 1));
        bar.addView(codeTab, new LinearLayout.LayoutParams(0, -2, 1));
        return bar;
    }

    private Button workshopButton(String title, boolean codeWorkshop, int highlight, int base) {
        Button tab = button(title, view -> select(codeWorkshop));
        tab.setMinHeight(dp(48));
        tab.setMinimumHeight(dp(48));
        tab.setPadding(dp(8), dp(7), dp(8), dp(7));
        tab.setTextColor(new ColorStateList(
                new int[][]{{-android.R.attr.state_enabled}, {android.R.attr.state_selected}, {}},
                new int[]{0xffa49bb4, Color.WHITE, 0xffe4ddef}));
        tab.setBackgroundTintList(null);
        tab.setStateListAnimator(null);
        StateListDrawable surface = new StateListDrawable();
        surface.addState(new int[]{android.R.attr.state_selected}, glassGradient(18, highlight, base));
        surface.addState(new int[]{}, glassGradient(18, Color.TRANSPARENT, Color.TRANSPARENT));
        Drawable mask = glassGradient(18, Color.WHITE, Color.WHITE);
        tab.setBackground(new RippleDrawable(ColorStateList.valueOf(0x36ffffff), surface, mask));
        return tab;
    }

    private GradientDrawable glassGradient(int radius, int... colors) {
        GradientDrawable surface = new GradientDrawable(GradientDrawable.Orientation.TL_BR, colors);
        surface.setCornerRadius(dp(radius));
        return surface;
    }

    private void select(boolean codeWorkshop) {
        if(code!=codeWorkshop){thread=null;output.setText("");prompt.setText("");}code = codeWorkshop;
        textTab.setSelected(!code);
        codeTab.setSelected(code);
        textTab.setEnabled(!busy);
        codeTab.setEnabled(!busy);
    }
    private boolean online(){ConnectivityManager cm=(ConnectivityManager)getSystemService(Context.CONNECTIVITY_SERVICE);NetworkCapabilities caps=cm.getNetworkCapabilities(cm.getActiveNetwork());return caps!=null&&caps.hasCapability(NetworkCapabilities.NET_CAPABILITY_VALIDATED);}
    private void refreshNetwork(){connection.setText(online()?"En ligne · calcul toujours local":"Hors ligne · modèles installés utilisables");}
    @Override protected void onResume(){super.onResume();if(connection!=null)refreshNetwork();}
    private void refresh(){modelLabel.setText(model==null?"Aucun modèle installé":display(model)+" · "+(model.length()/1048576)+" Mio");run.setEnabled(nativeReady&&!busy&&model!=null);pick.setEnabled(!busy);download.setEnabled(!busy);stop.setEnabled(busy);select(code);if(!nativeReady)state.setText("Moteur natif non chargé. La génération est indisponible.");}
    @Override protected void onActivityResult(int req,int result,Intent data){super.onActivityResult(req,result,data);if((req==41||req==42)&&result==RESULT_OK&&data!=null&&data.getData()!=null&&!busy){
        Uri uri=data.getData();busy=true;cancelled.set(false);refresh();state.setText("Lecture locale en cours…");
        worker.submit(()->{try{
            if(req==42){Attachments.installSpeech(this,uri,cancelled::get);runOnUiThread(()->finishJob("Modèle vocal installé."));return;}
            String mime=getContentResolver().getType(uri);String content;
            if(mime!=null&&mime.startsWith("image/"))content=Attachments.image(this,uri);
            else if(mime!=null&&(mime.startsWith("audio/")||mime.startsWith("video/")))content=Attachments.audio(this,uri,cancelled::get);
            else try(InputStream in=getContentResolver().openInputStream(uri)){
                if(in==null)throw new IOException("Document inaccessible");
                ByteArrayOutputStream bytes=new ByteArrayOutputStream();byte[] buf=new byte[2048];int n;
                while((n=in.read(buf))!=-1){if(bytes.size()+n>24000)throw new IOException("Document trop long");bytes.write(buf,0,n);}
                content=StandardCharsets.UTF_8.newDecoder().decode(java.nio.ByteBuffer.wrap(bytes.toByteArray())).toString();
            }
            String added=content.trim();
            if(added.isEmpty())throw new IOException("Aucun texte extrait.");
            runOnUiThread(()->{if(isDestroyed())return;if(!cancelled.get()&&added.length()+prompt.length()+1<=6000){prompt.append("\n"+added);finishJob("Pièce jointe intégrée.");}else finishJob("Import annulé ou contexte mobile dépassé (6 000 caractères).");});
        }catch(Exception e){runOnUiThread(()->finishJob("Pièce jointe · "+e.getMessage()));}});return;
    }if(req!=PICK||result!=RESULT_OK||data==null||data.getData()==null||busy)return;Uri uri=data.getData();busy=true;cancelled.set(false);refresh();state.setText("Import local en cours…");worker.submit(()->{try{InputStream in=getContentResolver().openInputStream(uri);if(in==null)throw new IOException("Fichier inaccessible");String displayName=uri.getLastPathSegment();try(android.database.Cursor cursor=getContentResolver().query(uri,new String[]{android.provider.OpenableColumns.DISPLAY_NAME},null,null,null)){if(cursor!=null&&cursor.moveToFirst())displayName=cursor.getString(0);}final String filename=displayName;File dest=ModelFiles.copy(in,new File(getFilesDir(),"models"),cancelled::get);runOnUiThread(()->{if(isDestroyed())return;model=dest;getPreferences(0).edit().putString("model",dest.getName()).putString("name-"+dest.getName(),filename).apply();finishJob("Modèle importé.");});}catch(Exception e){runOnUiThread(()->finishJob("Import : "+e.getMessage()));}});}
    private void start(boolean test){if(busy||!nativeReady||model==null)return;String q=test?"Réponds OK.":prompt.getText().toString().trim();if(q.isEmpty()){state.setText("Écris une demande.");return;}if(q.length()>6000){state.setText("Demande trop longue pour ce profil.");return;}File selected=model;String past=thread==null||test?"":conversations.read(thread);
    String recent=past.substring(Math.max(0,past.length()-2200));
    String request=recent+(code?"Réponds avec du code et des explications utiles :\n"+q:q);busy=true;cancelled.set(false);resetStop();refresh();output.setText("");state.setText("Calcul local… (aucune requête Internet)");worker.submit(()->{try{byte[] result=generateNative(selected.getAbsolutePath(),request.getBytes(StandardCharsets.UTF_8),384,Math.min(4,Runtime.getRuntime().availableProcessors()));String answer=new String(result,StandardCharsets.UTF_8);runOnUiThread(()->{if(isDestroyed())return;if(!test){
                    if(thread==null)thread=java.util.UUID.randomUUID().toString();
                    conversations.add(category(),thread,q,answer);output.setText(conversations.read(thread));
                }else output.setText(answer);
                finishJob(test?"Test local réussi.":"Génération terminée · modèle déchargé.");});}catch(Exception e){runOnUiThread(()->finishJob("Génération : "+e.getMessage()));}});}
    private void finishJob(String s){if(isDestroyed())return;busy=false;state.setText(s);refresh();}
    @Override protected void onDestroy(){cancelled.set(true);if(nativeReady)stopNative();worker.shutdownNow();super.onDestroy();}
}
